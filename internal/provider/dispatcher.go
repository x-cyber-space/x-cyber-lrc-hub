package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/scoring"
)

const (
	// searchTimeout bounds the parallel provider lookup.
	searchTimeout = 6 * time.Second
	// fetchTimeout bounds lyric retrieval. It is deliberately independent of
	// searchTimeout: a slow search used to eat the same budget as the fetch
	// phase, so a 5s search left 1s to collect lyrics from six candidates and
	// produced phantom "no lyrics" results.
	fetchTimeout = 8 * time.Second

	// searchPerProvider is how many candidates each platform returns.
	searchPerProvider = 5

	// maxGetFetches caps lyric fetches on the strict /api/get path.
	maxGetFetches = 6
	// maxSearchFetches mirrors lrclib.net, whose /api/search returns up to 20
	// records with their lyrics already embedded.
	maxSearchFetches = 20
	// fetchConcurrency bounds simultaneous outbound lyric requests.
	fetchConcurrency = 8

	// searchScoreFloor filters the ranked candidate list returned by
	// /api/search. It is intentionally *below* the app-side 60 line: the
	// consuming client re-ranks with its own model and renders a colour-coded
	// candidate list, so the server's job is recall, not judgement.
	searchScoreFloor = 40.0
	// fetchPrefilterScore is a purely economic cut: below it a candidate is
	// not worth spending an outbound lyric request on.
	fetchPrefilterScore = 30.0
)

// Dispatcher coordinates concurrent searches across all music providers.
type Dispatcher struct {
	providers []Provider
}

// NewDispatcher creates a dispatcher initialized with all standard providers.
func NewDispatcher() *Dispatcher {
	return NewDispatcherWith(
		NewNetEaseProvider(),
		NewQQMusicProvider(),
		NewKugouProvider(),
		NewKuwoProvider(),
	)
}

// NewDispatcherWith builds a dispatcher over an explicit provider set.
//
// It exists so the matching and caching behaviour can be tested offline with
// stub providers — the layer that previously had no coverage, and where every
// matching bug in this project lived.
func NewDispatcherWith(providers ...Provider) *Dispatcher {
	return &Dispatcher{providers: providers}
}

// rankedCandidate pairs a candidate with how it matched the query.
type rankedCandidate struct {
	cand  *model.Candidate
	stage scoring.MatchStage
}

// Get resolves a single confident match, mirroring lrclib.net's /api/get.
//
// The path is a hard filter, not a ranking: a candidate must satisfy the
// normalised identity (and the album/duration filters when supplied) either
// exactly or under the controlled relaxation in scoring.MatchTrack. Surviving
// candidates are ordered by strictness, then duration proximity, then the
// fuzzy score as a final tie-break.
//
// It returns (nil, nil) when nothing qualifies. Callers must NOT fall back to
// the best-scoring candidate here: an unrelated artist scoring 45 purely on a
// title collision is a wrong lyric, and a wrong lyric is indistinguishable
// from a right one once a player renders it. A miss is recoverable — the
// client can retry through /api/search — a false hit is not.
func (d *Dispatcher) Get(ctx context.Context, q model.Query) (*model.Candidate, error) {
	all, err := d.searchAll(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}

	matched := make([]rankedCandidate, 0, len(all))
	for _, cand := range all {
		stage := scoring.MatchTrack(q, cand.TrackName, cand.ArtistName, cand.AlbumName, cand.Duration)
		if stage == scoring.MatchNone {
			continue
		}
		// The fuzzy score is not a gate here; it only breaks ties.
		cand.Score = scoring.CalculateScore(q.TrackName, q.ArtistName, q.Duration,
			cand.TrackName, cand.ArtistName, cand.Duration)
		matched = append(matched, rankedCandidate{cand: cand, stage: stage})
	}
	if len(matched) == 0 {
		return nil, nil
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].stage != matched[j].stage {
			return matched[i].stage > matched[j].stage // MatchExact first
		}
		di := scoring.DurationDistance(q.Duration, matched[i].cand.Duration)
		dj := scoring.DurationDistance(q.Duration, matched[j].cand.Duration)
		if di != dj {
			return di < dj
		}
		return matched[i].cand.Score > matched[j].cand.Score
	})

	matched = dedupeRanked(matched)
	if len(matched) > maxGetFetches {
		matched = matched[:maxGetFetches]
	}

	fetched := d.fetchLyrics(ctx, matched)
	if len(fetched) == 0 {
		return nil, nil
	}
	return fetched[0].cand, nil
}

// Search returns a fuzzy-ranked candidate list, mirroring lrclib.net's
// /api/search.
//
// Unlike Get this is deliberately permissive: candidates are ranked by the
// weighted score and filtered only at searchScoreFloor. The intelligence
// belongs downstream — in a client that can combine this list with its own
// model and, ultimately, a human choosing from a rendered list.
func (d *Dispatcher) Search(ctx context.Context, q model.Query) ([]*model.Candidate, error) {
	all, err := d.searchAll(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}

	for _, cand := range all {
		cand.Score = scoring.CalculateScore(q.TrackName, q.ArtistName, q.Duration,
			cand.TrackName, cand.ArtistName, cand.Duration)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })

	shortlist := make([]*model.Candidate, 0, len(all))
	for _, cand := range dedupeCandidates(all) {
		if cand.Score >= fetchPrefilterScore {
			shortlist = append(shortlist, cand)
		}
	}
	if len(shortlist) > maxSearchFetches {
		shortlist = shortlist[:maxSearchFetches]
	}

	items := make([]rankedCandidate, 0, len(shortlist))
	for _, cand := range shortlist {
		stage := scoring.MatchTrack(q, cand.TrackName, cand.ArtistName, cand.AlbumName, cand.Duration)
		items = append(items, rankedCandidate{cand: cand, stage: stage})
	}

	fetched := d.fetchLyrics(ctx, items)

	out := make([]*model.Candidate, 0, len(fetched))
	for _, item := range fetched {
		if item.cand.Score >= searchScoreFloor {
			out = append(out, item.cand)
		}
	}
	return out, nil
}

// searchAll queries every provider in parallel and returns the union.
//
// An error is reported only when every provider failed, which is the one case
// the caller must not mistake for "no lyrics exist".
func (d *Dispatcher) searchAll(ctx context.Context, q model.Query) ([]*model.Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	keyword := buildKeyword(q)

	var (
		mu       sync.Mutex
		all      []*model.Candidate
		failures int
		wg       sync.WaitGroup
	)

	for _, prov := range d.providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()

			results, err := p.Search(ctx, keyword, searchPerProvider)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures++
				return
			}
			all = append(all, results...)
		}(prov)
	}
	wg.Wait()

	if len(all) == 0 && failures == len(d.providers) {
		return nil, fmt.Errorf("all %d providers failed", failures)
	}
	return all, nil
}

// buildKeyword composes the provider search keyword.
func buildKeyword(q model.Query) string {
	parts := []string{strings.TrimSpace(q.TrackName)}
	if !scoring.IsUnknownArtist(q.ArtistName) {
		parts = append(parts, strings.TrimSpace(q.ArtistName))
	}
	// Join-then-trim avoids the leading space an empty title used to leave
	// behind when only an artist was supplied.
	return strings.TrimSpace(strings.Join(parts, " "))
}

// fetchLyrics retrieves lyrics for the given candidates concurrently and
// returns the ones that yielded usable content, preserving the input order.
func (d *Dispatcher) fetchLyrics(ctx context.Context, items []rankedCandidate) []rankedCandidate {
	if len(items) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	byName := make(map[string]Provider, len(d.providers))
	for _, p := range d.providers {
		byName[p.Name()] = p
	}

	usable := make([]bool, len(items))
	sem := make(chan struct{}, fetchConcurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		prov, ok := byName[item.cand.Source]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(idx int, cand *model.Candidate, p Provider) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := p.FetchLyrics(ctx, cand); err != nil {
				return
			}
			// A candidate is only usable if it carries lyrics, or is a
			// genuine instrumental (where empty lyrics are the answer).
			if cand.SyncedLyrics == "" && cand.PlainLyrics == "" && !cand.Instrumental {
				return
			}
			usable[idx] = true
		}(i, item.cand, prov)
	}
	wg.Wait()

	out := make([]rankedCandidate, 0, len(items))
	for i, item := range items {
		if usable[i] {
			out = append(out, item)
		}
	}
	return out
}

// identityKey collapses the several platforms that offer the same recording.
//
// It delegates to scoring.IdentityKey so that deduplication and the cache key
// agree by construction: if two candidates collapse into one result here, they
// also occupy one cache row, and the two can never disagree about whether they
// are the same edit.
func identityKey(trackName, artistName string, duration float64) string {
	return scoring.IdentityKey(trackName, artistName, duration)
}

func dedupeRanked(items []rankedCandidate) []rankedCandidate {
	seen := make(map[string]struct{}, len(items))
	out := make([]rankedCandidate, 0, len(items))
	for _, item := range items {
		key := identityKey(item.cand.TrackName, item.cand.ArtistName, item.cand.Duration)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeCandidates(items []*model.Candidate) []*model.Candidate {
	seen := make(map[string]struct{}, len(items))
	out := make([]*model.Candidate, 0, len(items))
	for _, item := range items {
		key := identityKey(item.TrackName, item.ArtistName, item.Duration)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
