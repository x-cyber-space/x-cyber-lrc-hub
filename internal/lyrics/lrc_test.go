package lyrics

import (
	"strings"
	"testing"
)

func TestExtractPlainLyrics(t *testing.T) {
	synced := `[ti:七里香]
[ar:周杰伦]
[al:七里香]
[00:00.00] 作词 : 方文山
[00:02.00] 作曲 : 周杰伦
[00:18.50]窗外的麻雀 在电线杆上多嘴
[00:22.80]你说这一句 很有夏天的感觉`

	plain := ExtractPlainLyrics(synced)
	expected := "作词 : 方文山\n作曲 : 周杰伦\n窗外的麻雀 在电线杆上多嘴\n你说这一句 很有夏天的感觉"
	if plain != expected {
		t.Fatalf("Expected:\n%s\nGot:\n%s", expected, plain)
	}
}

func TestIsInstrumental(t *testing.T) {
	if !IsInstrumental("纯音乐，请欣赏", "[00:00.00] 纯音乐，请欣赏") {
		t.Errorf("Expected true for instrumental")
	}

	if IsInstrumental("窗外的麻雀 在电线杆上多嘴", "[00:18.50]窗外的麻雀 在电线杆上多嘴") {
		t.Errorf("Expected false for non-instrumental")
	}
}

func TestFormatSynced(t *testing.T) {
	lines := []TimedLine{
		{Start: 0, Text: "七里香 - 周杰伦"},
		{Start: 18.5, Text: "窗外的麻雀 在电线杆上多嘴"},
	}

	synced, plain := FormatSynced(lines)
	if !strings.Contains(synced, "[00:18.50] 窗外的麻雀 在电线杆上多嘴") {
		t.Errorf("Unexpected synced output: %s", synced)
	}
	if !strings.Contains(plain, "窗外的麻雀 在电线杆上多嘴") {
		t.Errorf("Unexpected plain output: %s", plain)
	}
}

// TestFormatSyncedKeepsOutputsInLockstep matters because the two forms are
// served side by side: line N of the plain text must be line N of the synced
// text, so a skipped blank line has to be skipped by both.
func TestFormatSyncedKeepsOutputsInLockstep(t *testing.T) {
	synced, plain := FormatSynced([]TimedLine{
		{Start: 1, Text: "first"},
		{Start: 2, Text: "   "}, // dropped
		{Start: 3, Text: "second"},
		{Start: 4, Text: ""}, // dropped
		{Start: 5, Text: "third"},
	})

	if got, want := len(strings.Split(synced, "\n")), 3; got != want {
		t.Fatalf("synced has %d lines, want %d:\n%s", got, want, synced)
	}
	if got, want := len(strings.Split(plain, "\n")), 3; got != want {
		t.Fatalf("plain has %d lines, want %d:\n%s", got, want, plain)
	}
	if !strings.HasSuffix(synced, "[00:05.00] third") {
		t.Errorf("unexpected synced output: %s", synced)
	}
}

func TestFormatSyncedClampsNegativeOffset(t *testing.T) {
	synced, _ := FormatSynced([]TimedLine{{Start: -3, Text: "clamped"}})
	if !strings.HasPrefix(synced, "[00:00.00]") {
		t.Errorf("a negative offset must clamp to zero, got %s", synced)
	}
}

// TestFormatSyncedMinutesRollover guards the mm:ss rollover, which is easy to
// get wrong when the offset is only ever formatted, never parsed back.
func TestFormatSyncedMinutesRollover(t *testing.T) {
	synced, _ := FormatSynced([]TimedLine{{Start: 125.25, Text: "late"}})
	if !strings.HasPrefix(synced, "[02:05.25]") {
		t.Errorf("expected [02:05.25] for 125.25s, got %s", synced)
	}
}
