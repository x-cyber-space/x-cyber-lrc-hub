package matching

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
