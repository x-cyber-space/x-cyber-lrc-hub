package provider

import (
	"context"
	"testing"
	"time"
)

func TestProvidersSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dispatcher := NewDispatcher()
	best, list, err := dispatcher.SearchAndScore(ctx, "七里香", "周杰伦", "七里香", 299.0, 50.0)
	if err != nil {
		t.Fatalf("SearchAndScore failed: %v", err)
	}

	if best == nil {
		t.Logf("Warning: No best candidate found (network/proxy issue?), but error was nil")
		return
	}

	if best.Score < 60.0 {
		t.Errorf("Expected high score for 七里香, got %f", best.Score)
	}

	if best.SyncedLyrics == "" && best.PlainLyrics == "" && !best.Instrumental {
		t.Errorf("Candidate has no lyrics and not instrumental")
	}

	t.Logf("Successfully fetched best: %s - %s from %s (Score: %.2f)",
		best.TrackName, best.ArtistName, best.Source, best.Score)
	t.Logf("Total valid candidates with lyrics: %d", len(list))
}
