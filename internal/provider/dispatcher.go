package provider

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/scoring"
)

// Dispatcher coordinates concurrent searches across all music providers
type Dispatcher struct {
	providers []Provider
}

// NewDispatcher creates a dispatcher initialized with all standard providers
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		providers: []Provider{
			NewNetEaseProvider(),
			NewQQMusicProvider(),
			NewKugouProvider(),
			NewKuwoProvider(),
		},
	}
}

// SearchAndScore queries all providers in parallel, fetches lyrics for top candidates,
// and returns the best candidate along with the sorted candidate list.
func (d *Dispatcher) SearchAndScore(ctx context.Context, targetTitle, targetArtist, targetAlbum string, targetDuration float64, minScore float64) (*model.Candidate, []*model.Candidate, error) {
	// Build search query keyword
	query := strings.TrimSpace(targetTitle)
	if !scoring.IsUnknownArtist(targetArtist) && strings.TrimSpace(targetArtist) != "" {
		query = query + " " + strings.TrimSpace(targetArtist)
	}

	searchCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	var (
		mu         sync.Mutex
		candidates []*model.Candidate
		wg         sync.WaitGroup
	)

	// Step 1: Parallel search across all providers
	for _, p := range d.providers {
		wg.Add(1)
		go func(prov Provider) {
			defer wg.Done()
			results, err := prov.Search(searchCtx, query, 4)
			if err != nil || len(results) == 0 {
				return
			}

			mu.Lock()
			candidates = append(candidates, results...)
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	if len(candidates) == 0 {
		return nil, nil, nil
	}

	// Step 2: Preliminarily score candidates and filter top ones for lyric retrieval
	for _, c := range candidates {
		c.Score = scoring.CalculateScore(targetTitle, targetArtist, targetDuration, c.TrackName, c.ArtistName, c.Duration)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Select top candidates (up to 6) with score >= 35 to fetch lyrics
	var toFetch []*model.Candidate
	for _, c := range candidates {
		if c.Score >= 35.0 && len(toFetch) < 6 {
			toFetch = append(toFetch, c)
		}
	}
	if len(toFetch) == 0 && len(candidates) > 0 {
		toFetch = append(toFetch, candidates[0])
	}

	// Step 3: Concurrently fetch lyrics for selected candidates
	var (
		lyricWg sync.WaitGroup
		validMu sync.Mutex
		valid   []*model.Candidate
	)

	providerMap := make(map[string]Provider)
	for _, p := range d.providers {
		providerMap[p.Name()] = p
	}

	for _, cand := range toFetch {
		prov, ok := providerMap[cand.Source]
		if !ok {
			continue
		}

		lyricWg.Add(1)
		go func(c *model.Candidate, p Provider) {
			defer lyricWg.Done()
			err := p.FetchLyrics(searchCtx, c)
			if err != nil {
				return
			}

			// Verify that either lyrics exist or it's an instrumental track
			if c.SyncedLyrics == "" && c.PlainLyrics == "" && !c.Instrumental {
				return
			}

			validMu.Lock()
			valid = append(valid, c)
			validMu.Unlock()
		}(cand, prov)
	}
	lyricWg.Wait()

	if len(valid) == 0 {
		return nil, nil, nil
	}

	// Sort valid candidates with lyrics by score descending
	sort.Slice(valid, func(i, j int) bool {
		return valid[i].Score > valid[j].Score
	})

	var best *model.Candidate
	if valid[0].Score >= minScore {
		best = valid[0]
	}

	return best, valid, nil
}
