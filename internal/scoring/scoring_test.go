package scoring

import (
	"testing"
)

func TestNormalizeText(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"七里香", "七里香"},
		{"七里香 (Live)", "七里香"},
		{"七里香 [Remix]", "七里香"},
		{"七里香（伴奏）", "七里香"},
		{"【超清】七里香", "七里香"},
		{"In the End (feat. Jay-Z)", "intheend"},
		{"晴天 (DJ版)", "晴天"},
	}

	for _, c := range cases {
		got := NormalizeText(c.input)
		if got != c.expected {
			t.Errorf("NormalizeText(%q) = %q, expected %q", c.input, got, c.expected)
		}
	}
}

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
