package scoring

import (
	"testing"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/matching"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

func TestCalculateScore(t *testing.T) {
	// Exact match with duration < 1s
	score := CalculateScore("七里香", "周杰伦", 299.0, "七里香 (Live)", "周杰伦", 299.5)
	// Title exact (after norm): 45, Artist exact: 30, Duration diff 0.5s: 25. Total = 100
	if score != 100.0 {
		t.Errorf("Expected 100.0, got %f", score)
	}

	// Unknown artist case
	scoreUnknown := CalculateScore("七里香", "未知歌手", 299.0, "七里香", "周杰伦", 299.0)
	// Title: 45, Artist unknown: 20, Duration: 25. Total = 90
	if scoreUnknown != 90.0 {
		t.Errorf("Expected 90.0, got %f", scoreUnknown)
	}

	// Duration diff >= 5s
	scoreDiffDur := CalculateScore("七里香", "周杰伦", 300.0, "七里香", "周杰伦", 290.0)
	// Title: 45, Artist: 30, Duration: 0. Total = 75
	if scoreDiffDur != 75.0 {
		t.Errorf("Expected 75.0, got %f", scoreDiffDur)
	}
}

// TestCalculateScoreContract pins the fuzzy scorer's role: it ranks, it does
// not gate. A perfect title with an unrelated artist still scores highly here,
// which is precisely why /api/get must not use it as an acceptance threshold.
func TestCalculateScoreContract(t *testing.T) {
	score := CalculateScore("童话", "光良", 246, "童话", "王菲", 255)
	if score < 40 {
		t.Fatalf("expected the additive scorer to rate a title collision highly, got %.2f", score)
	}
	if matching.MatchTrack(
		model.Query{TrackName: "童话", ArtistName: "光良", Duration: 246},
		"童话", "王菲", "", 255,
	) != matching.MatchNone {
		t.Fatal("the matcher must reject what the scorer rates highly")
	}
}
