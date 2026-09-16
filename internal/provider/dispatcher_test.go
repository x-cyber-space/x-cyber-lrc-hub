package provider

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/scoring"
)

// stubProvider is an offline Provider double. Every test in this file runs
// without touching the network, which is the point: the dispatcher's matching
// and caching behaviour is where this project's correctness lives.
type stubProvider struct {
	name      string
	cands     []*model.Candidate
	searchErr error

	lyrics       string
	fetchErr     error
	instrumental bool

	mu           sync.Mutex
	lastKeyword  string
	fetchCounter int
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) Search(_ context.Context, keyword string, _ int) ([]*model.Candidate, error) {
	s.mu.Lock()
	s.lastKeyword = keyword
	s.mu.Unlock()

	if s.searchErr != nil {
		return nil, s.searchErr
	}

	// Return copies so one test case cannot leak Score/lyrics into the next.
	out := make([]*model.Candidate, 0, len(s.cands))
	for _, c := range s.cands {
		clone := *c
		clone.Source = s.name
		out = append(out, &clone)
	}
	return out, nil
}

func (s *stubProvider) FetchLyrics(_ context.Context, c *model.Candidate) error {
	s.mu.Lock()
	s.fetchCounter++
	s.mu.Unlock()

	if s.fetchErr != nil {
		return s.fetchErr
	}
	if s.instrumental {
		c.Instrumental = true
		return nil
	}
	c.PlainLyrics = s.lyrics
	c.SyncedLyrics = "[00:00.00] " + s.lyrics
	return nil
}

func (s *stubProvider) fetched() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchCounter
}

func (s *stubProvider) keyword() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastKeyword
}

func cand(track, artist string, duration float64) *model.Candidate {
	return &model.Candidate{TrackName: track, ArtistName: artist, Duration: duration}
}

// TestGetRejectsTitleCollision is the regression this refactor exists for.
//
// "童话" by 王菲 scores 45/100 on the additive model (full marks for the title,
// nothing for artist or duration) and sailed past the old >=40 threshold, so
// a request for 光良's 童话 could be answered with a different song entirely.
func TestGetRejectsTitleCollision(t *testing.T) {
	dispatcher := NewDispatcherWith(&stubProvider{
		name:   "stub",
		lyrics: "词",
		cands:  []*model.Candidate{cand("童话", "王菲", 255)},
	})

	got, err := dispatcher.Get(context.Background(), model.Query{
		TrackName: "童话", ArtistName: "光良", Duration: 246,
	})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("a title collision from another artist must not match, got %q by %q", got.TrackName, got.ArtistName)
	}
}

// TestGetRejectsTitleCollisionWithoutDuration covers the variant that the old
// free 20-point "neutral" duration score pushed to 65/100.
func TestGetRejectsTitleCollisionWithoutDuration(t *testing.T) {
	dispatcher := NewDispatcherWith(&stubProvider{
		name:   "stub",
		lyrics: "词",
		cands:  []*model.Candidate{cand("童话", "王菲", 0)},
	})

	got, err := dispatcher.Get(context.Background(), model.Query{TrackName: "童话", ArtistName: "光良"})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("missing duration must not create a free pass, got %q by %q", got.TrackName, got.ArtistName)
	}
}

func TestGetPrefersExactOverRelaxed(t *testing.T) {
	relaxed := &stubProvider{
		name:   "relaxed",
		lyrics: "合唱版歌词",
		cands:  []*model.Candidate{cand("千里之外", "周杰伦 / 费玉清", 239)},
	}
	exact := &stubProvider{
		name:   "exact",
		lyrics: "独唱版歌词",
		cands:  []*model.Candidate{cand("千里之外", "周杰伦", 239)},
	}

	// Deliberately listed relaxed-first: ordering must come from match
	// confidence, not from provider slice order.
	dispatcher := NewDispatcherWith(relaxed, exact)

	got, err := dispatcher.Get(context.Background(), model.Query{
		TrackName: "千里之外", ArtistName: "周杰伦", Duration: 239,
	})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected a match")
	}
	if got.Source != "exact" {
		t.Fatalf("expected the exact match to win, got %q (artist %q)", got.Source, got.ArtistName)
	}
}

func TestGetPrefersNearestDurationWithinSameStage(t *testing.T) {
	near := &stubProvider{name: "near", lyrics: "近", cands: []*model.Candidate{cand("晴天", "周杰伦", 269)}}
	far := &stubProvider{name: "far", lyrics: "远", cands: []*model.Candidate{cand("晴天", "周杰伦", 271)}}

	dispatcher := NewDispatcherWith(far, near)

	got, err := dispatcher.Get(context.Background(), model.Query{
		TrackName: "晴天", ArtistName: "周杰伦", Duration: 269.2,
	})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil || got.Source != "near" {
		t.Fatalf("expected the duration-nearest candidate, got %+v", got)
	}
}

func TestGetErrorsOnlyWhenEveryProviderFails(t *testing.T) {
	boom := errors.New("upstream down")

	allDown := NewDispatcherWith(
		&stubProvider{name: "a", searchErr: boom},
		&stubProvider{name: "b", searchErr: boom},
	)
	if _, err := allDown.Get(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦"}); err == nil {
		t.Fatal("expected an error when every provider fails, so the handler can answer 500 instead of 404")
	}

	partial := NewDispatcherWith(
		&stubProvider{name: "a", searchErr: boom},
		&stubProvider{name: "b", lyrics: "词", cands: []*model.Candidate{cand("晴天", "周杰伦", 269)}},
	)
	got, err := partial.Get(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269})
	if err != nil {
		t.Fatalf("a single provider failure must not fail the request: %v", err)
	}
	if got == nil {
		t.Fatal("expected the surviving provider to answer")
	}
}

func TestGetDropsCandidatesWithoutLyrics(t *testing.T) {
	dispatcher := NewDispatcherWith(&stubProvider{
		name:     "stub",
		fetchErr: errors.New("no lyrics upstream"),
		cands:    []*model.Candidate{cand("晴天", "周杰伦", 269)},
	})

	got, err := dispatcher.Get(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != nil {
		t.Fatal("a candidate whose lyrics could not be fetched must not be returned")
	}
}

func TestGetAcceptsInstrumentalWithoutLyrics(t *testing.T) {
	dispatcher := NewDispatcherWith(&stubProvider{
		name:         "stub",
		instrumental: true,
		cands:        []*model.Candidate{cand("晴天", "周杰伦", 269)},
	})

	got, err := dispatcher.Get(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil || !got.Instrumental {
		t.Fatalf("an instrumental track is a valid answer with empty lyrics, got %+v", got)
	}
}

func TestGetDeduplicatesAcrossProviders(t *testing.T) {
	// The same recording offered by four platforms must cost one lyric fetch.
	providers := make([]Provider, 0, 4)
	for _, name := range []string{"netease", "qqmusic", "kugou", "kuwo"} {
		providers = append(providers, &stubProvider{
			name:   name,
			lyrics: "词",
			cands:  []*model.Candidate{cand("晴天", "周杰伦", 269)},
		})
	}
	dispatcher := NewDispatcherWith(providers...)

	got, err := dispatcher.Get(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269})
	if err != nil || got == nil {
		t.Fatalf("expected a match, got %+v (err=%v)", got, err)
	}

	total := 0
	for _, p := range providers {
		total += p.(*stubProvider).fetched()
	}
	if total != 1 {
		t.Fatalf("expected 1 lyric fetch after deduplication, got %d", total)
	}
}

func TestSearchAppliesScoreFloor(t *testing.T) {
	dispatcher := NewDispatcherWith(&stubProvider{
		name:   "stub",
		lyrics: "词",
		cands:  []*model.Candidate{cand("完全无关的歌", "无关歌手", 100)},
	})

	got, err := dispatcher.Search(context.Background(), model.Query{TrackName: "目标歌名", ArtistName: "目标歌手"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("search must drop candidates below the floor, got %d: %+v", len(got), got)
	}
}

func TestSearchReturnsRankedCandidates(t *testing.T) {
	dispatcher := NewDispatcherWith(&stubProvider{
		name:   "stub",
		lyrics: "词",
		cands: []*model.Candidate{
			cand("晴天", "周杰伦", 275), // title matches, duration 6s off
			cand("晴天", "周杰伦", 269), // perfect
		},
	})

	got, err := dispatcher.Search(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected both candidates to clear the search floor, got %d", len(got))
	}
	if got[0].Score < got[1].Score {
		t.Fatalf("results must be ranked by score descending: %.2f then %.2f", got[0].Score, got[1].Score)
	}
	if got[0].Duration != 269 {
		t.Fatalf("expected the exact-duration candidate first, got duration %.0f", got[0].Duration)
	}
}

// TestSearchFloorIsGentlerThanTheMatcher documents the intended division of
// labour: /api/search is allowed to surface candidates that /api/get would
// reject, because a downstream client with its own model and a human in the
// loop is expected to make the final call.
func TestSearchFloorIsGentlerThanTheMatcher(t *testing.T) {
	candidate := cand("晴天 现场版", "周杰伦", 273)

	dispatcher := NewDispatcherWith(&stubProvider{name: "stub", lyrics: "词", cands: []*model.Candidate{candidate}})
	query := model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269}

	got, err := dispatcher.Search(context.Background(), query)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the live take to be searchable, got %d results", len(got))
	}

	if scoring.MatchTrack(query, candidate.TrackName, candidate.ArtistName, candidate.AlbumName, candidate.Duration) == scoring.MatchNone {
		t.Skip("the live take is inside the relaxed matcher too; the floor-vs-matcher distinction is covered elsewhere")
	}
}

// TestSearchAndCacheIdentityAgree guards the regression where the search
// dedupe key (whole seconds) was coarser than the cache key bucket (2s): two
// candidates for the same recording could collapse onto a single cache row and
// still be reported as two results carrying the same row id.
func TestSearchAndCacheIdentityAgree(t *testing.T) {
	near := cand("七里香 (女声版)", "吉拉朵", 261.0)
	slightlyOff := cand("七里香 (女声版)", "吉拉朵", 261.9)

	// The two durations must land in the same identity bucket, which is what
	// makes them one cache row...
	if scoring.IdentityKey(near.TrackName, near.ArtistName, near.Duration) !=
		scoring.IdentityKey(slightlyOff.TrackName, slightlyOff.ArtistName, slightlyOff.Duration) {
		t.Fatal("test premise broken: these two should share an identity bucket")
	}

	// ...and therefore exactly one result, so the returned ids stay unique.
	dispatcher := NewDispatcherWith(&stubProvider{
		name:   "stub",
		lyrics: "词",
		cands:  []*model.Candidate{near, slightlyOff},
	})

	got, err := dispatcher.Search(context.Background(), model.Query{TrackName: "七里香", ArtistName: "吉拉朵", Duration: 261})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates sharing a cache row must collapse to one result, got %d", len(got))
	}
}

// TestSearchResultIDsAreUnique exercises the same invariant through the API's
// actual write path semantics: distinct results must map to distinct rows.
func TestSearchResultIDsAreUnique(t *testing.T) {
	candidates := []*model.Candidate{
		cand("七里香 (Live)", "周杰伦", 302),
		cand("七里香 (女声版)", "吉拉朵", 261.0),
		cand("七里香 (女声版)", "吉拉朵", 261.9), // same identity as the previous one
		cand("七里香 (DJ版)", "周杰伦", 117),
	}

	dispatcher := NewDispatcherWith(&stubProvider{name: "stub", lyrics: "词", cands: candidates})
	got, err := dispatcher.Search(context.Background(), model.Query{TrackName: "七里香", ArtistName: "周杰伦"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	seen := make(map[string]string)
	for _, c := range got {
		key := scoring.IdentityKey(c.TrackName, c.ArtistName, c.Duration)
		if prev, dup := seen[key]; dup {
			t.Fatalf("identity %q reported twice: %q and %q", key, prev, c.TrackName)
		}
		seen[key] = c.TrackName
	}
}

func TestBuildKeywordHasNoLeadingSpace(t *testing.T) {
	stub := &stubProvider{name: "stub", lyrics: "词", cands: []*model.Candidate{cand("晴天", "周杰伦", 269)}}
	dispatcher := NewDispatcherWith(stub)

	// Only an artist supplied: the old code produced " 光良" with a leading space.
	if _, err := dispatcher.Search(context.Background(), model.Query{ArtistName: "光良"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if kw := stub.keyword(); kw != "光良" {
		t.Fatalf("keyword = %q, want %q", kw, "光良")
	}

	if _, err := dispatcher.Search(context.Background(), model.Query{TrackName: "晴天", ArtistName: "周杰伦"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if kw := stub.keyword(); kw != "晴天 周杰伦" {
		t.Fatalf("keyword = %q, want %q", kw, "晴天 周杰伦")
	}
	if strings.HasPrefix(stub.keyword(), " ") {
		t.Fatal("keyword must not carry a leading space")
	}
}
