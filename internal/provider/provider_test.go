package provider

import (
	"context"
	"testing"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// TestLiveProvidersSmoke exercises the four real platform adapters.
//
// It needs outbound network access, so it is skipped in -short mode
// (`go test -short ./...`). Unlike the previous version it FAILS when no
// provider answers, rather than logging a warning and passing — a smoke test
// that cannot fail is not a test.
//
// Album is deliberately left out of the query. It is the field most likely to
// disagree between a listener's tags and a platform's metadata, so requiring
// it here would make the smoke test flap for reasons unrelated to whether the
// providers are reachable; album handling has unit coverage instead.
func TestLiveProvidersSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("live provider smoke test skipped in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dispatcher := NewDispatcher()
	query := model.Query{TrackName: "七里香", ArtistName: "周杰伦", Duration: 299}

	// 1. Broad search must reach the platforms and come back with candidates.
	candidates, err := dispatcher.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("no provider returned a usable candidate for 七里香 — check network access and the platform APIs")
	}
	t.Logf("search returned %d candidates; best %s - %s (score %.2f)",
		len(candidates), candidates[0].TrackName, candidates[0].ArtistName, candidates[0].Score)

	// 2. The strict path must resolve the same track end to end.
	best, err := dispatcher.Get(ctx, query)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if best == nil {
		t.Fatal("search found candidates but the strict matcher resolved none — check duration windows and the relaxed stage")
	}
	if best.SyncedLyrics == "" && best.PlainLyrics == "" && !best.Instrumental {
		t.Fatalf("candidate %q has neither lyrics nor an instrumental flag", best.TrackName)
	}

	t.Logf("matched %s - %s from %s (score %.2f)", best.TrackName, best.ArtistName, best.Source, best.Score)
}
