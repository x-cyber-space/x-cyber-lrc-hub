package provider

import (
	"testing"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/lyrics"
)

// TestTimedLines covers the boundary between Kuwo's wire shape and the shared
// LRC renderer. Kuwo is the only provider that reports the offset as a string,
// so the parsing lives here and the failure mode matters: a dropped line
// silently removes a lyric from the middle of a song.
func TestTimedLines(t *testing.T) {
	got := timedLines([]kuwoLrcItem{
		{LineLyric: "第一行", Time: "0.0"},
		{LineLyric: "第二行", Time: "18.5"},
		{LineLyric: "坏时间戳", Time: "not-a-number"},
	})

	if len(got) != 3 {
		t.Fatalf("every item must survive conversion, got %d of 3", len(got))
	}
	if got[0].Start != 0 || got[1].Start != 18.5 {
		t.Errorf("unexpected offsets: %+v", got)
	}
	if got[2].Start != 0 {
		t.Errorf("an unparseable offset must fall back to 0, got %v", got[2].Start)
	}

	// And they still render.
	synced, plain := lyrics.FormatSynced(got)
	if synced == "" || plain == "" {
		t.Fatalf("converted lines did not render: synced=%q plain=%q", synced, plain)
	}
}

func TestTimedLinesEmpty(t *testing.T) {
	if got := timedLines(nil); len(got) != 0 {
		t.Fatalf("expected no lines, got %+v", got)
	}
}
