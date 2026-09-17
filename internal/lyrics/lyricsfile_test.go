package lyrics

import (
	"strings"
	"testing"
)

func TestBuildLyricsFileReturnsNilWhenEmpty(t *testing.T) {
	if got := BuildLyricsFile("T", "A", "Al", 100, true, "", ""); got != nil {
		t.Fatalf("expected nil so the field marshals as JSON null, got %q", *got)
	}
}

func TestBuildLyricsFileStructure(t *testing.T) {
	synced := `[ti:Wildest Dreams]
[ar:Taylor Swift]
[00:13.17]He said, "Let's get out of this town
[00:16.52]Drive out of the city, away from the crowds"
[00:20.06]I thought Heaven can't help me now`

	got := BuildLyricsFile("Wildest Dreams", "Taylor Swift", "1989", 220, false, synced, "He said\nDrive out")
	if got == nil {
		t.Fatal("expected a lyricsfile document")
	}

	// The header and its metadata block.
	for _, want := range []string{
		"version: '1.0'",
		"metadata:",
		"  duration_ms: 220000",
		"  instrumental: false",
		"lines:",
		"plain: |",
	} {
		if !strings.Contains(*got, want) {
			t.Errorf("missing %q in:\n%s", want, *got)
		}
	}

	// ALBUM must be quoted: it looks like a number, and `album: 1989` would
	// otherwise parse as an integer.
	if !strings.Contains(*got, "album: '1989'") {
		t.Errorf("numeric-looking album must be quoted:\n%s", *got)
	}

	// The title tag must not leak into the line list as text.
	if strings.Contains(*got, "[ti:") || strings.Contains(*got, "[ar:") {
		t.Errorf("metadata tags must be stripped from lines:\n%s", *got)
	}

	// Timestamps convert to milliseconds: 13.17s -> 13170ms.
	if !strings.Contains(*got, "start_ms: 13170") {
		t.Errorf("expected 13170ms for [00:13.17]:\n%s", *got)
	}
	if !strings.Contains(*got, "end_ms: 16520") {
		t.Errorf("expected end_ms to be the next line's start:\n%s", *got)
	}
}

// TestBuildLyricsFileQuotesAmbiguousText covers the reason the generator does
// not blindly emit plain scalars: Chinese lyrics are full of "作词 : 方文山",
// and an unquoted ": " turns the rest of the line into a nested mapping.
func TestBuildLyricsFileQuotesAmbiguousText(t *testing.T) {
	synced := "[00:00.00] 作词 : 方文山\n[00:02.00] 作曲 : 周杰伦"

	got := BuildLyricsFile("七里香", "周杰伦", "七里香", 299, false, synced, "作词 : 方文山")
	if got == nil {
		t.Fatal("expected a lyricsfile document")
	}

	if !strings.Contains(*got, `- text: '作词 : 方文山'`) {
		t.Errorf("text containing ': ' must be quoted:\n%s", *got)
	}
}

// TestBuildLyricsFileEscapesSingleQuotes covers the interaction between the
// two rules: a value is only quoted when it has to be, and once quoted its
// embedded single quotes must be doubled.
func TestBuildLyricsFileEscapesSingleQuotes(t *testing.T) {
	// ": " forces quoting; the apostrophe then has to be escaped.
	synced := "[00:00.00] Don't : stop"
	want := `- text: 'Don''t : stop'`

	got := BuildLyricsFile("T", "A", "", 100, false, synced, "Don't : stop")
	if got == nil {
		t.Fatal("expected a lyricsfile document")
	}
	if !strings.Contains(*got, want) {
		t.Errorf("expected %s in:\n%s", want, *got)
	}
}

// TestBuildLyricsFileLeavesSafeTextUnquoted documents that quoting is
// conditional: an apostrophe mid-value is legal in a YAML plain scalar, and
// lrclib.net emits such lines unquoted too.
func TestBuildLyricsFileLeavesSafeTextUnquoted(t *testing.T) {
	got := BuildLyricsFile("T", "A", "", 100, false, "[00:00.00] Don't stop", "Don't stop")
	if got == nil {
		t.Fatal("expected a lyricsfile document")
	}
	if !strings.Contains(*got, "- text: Don't stop") {
		t.Errorf("an apostrophe not at the start needs no quoting:\n%s", *got)
	}
}

func TestBuildLyricsFilePlainBlockIsIndented(t *testing.T) {
	got := BuildLyricsFile("T", "A", "Al", 100, false,
		"[00:00.00] first\n[00:05.00] second", "first\nsecond")
	if got == nil {
		t.Fatal("expected a lyricsfile document")
	}

	// Every line of the literal block must be indented under `plain: |`.
	idx := strings.Index(*got, "plain: |")
	if idx < 0 {
		t.Fatalf("missing plain block:\n%s", *got)
	}
	for _, line := range strings.Split(strings.TrimRight((*got)[idx:], "\n"), "\n")[1:] {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("plain block line is not indented: %q", line)
		}
	}
}

// TestParseTimedLinesHandlesMultipleTagsPerLine covers the LRC form where one
// text line carries several timestamps (a repeated chorus).
func TestParseTimedLinesHandlesMultipleTagsPerLine(t *testing.T) {
	got := parseTimedLines("[00:01.00][00:31.00] repeated chorus")
	if len(got) != 2 {
		t.Fatalf("expected 2 timed lines, got %d: %+v", len(got), got)
	}
	if got[0].startMs != 1000 || got[1].startMs != 31000 {
		t.Fatalf("unexpected timings: %+v", got)
	}
	for _, line := range got {
		if line.text != "repeated chorus" {
			t.Errorf("unexpected text %q", line.text)
		}
	}
}

func TestParseTimedLinesSortsByStart(t *testing.T) {
	got := parseTimedLines("[00:20.00] later\n[00:05.00] earlier")
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}
	if got[0].startMs != 5000 || got[1].startMs != 20000 {
		t.Fatalf("expected sorted output, got %+v", got)
	}
}
