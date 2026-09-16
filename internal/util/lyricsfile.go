package util

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LyricsFileVersion is the schema version emitted in the lyricsfile header.
const LyricsFileVersion = "1.0"

// lrcTimeTag captures one LRC timestamp with its components so it can be
// converted to milliseconds. LRC allows either "." or ":" before the
// fractional part.
var lrcTimeTag = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)

type timedLine struct {
	startMs int
	text    string
}

// BuildLyricsFile renders LRCLIB's `lyricsfile` field: a small YAML document
// carrying per-line millisecond timings alongside the plain text.
//
//	version: '1.0'
//	metadata:
//	  title: Wildest Dreams
//	  artist: Taylor Swift
//	  album: '1989'
//	  duration_ms: 220000
//	  instrumental: false
//	lines:
//	- text: He said, "Let's get out of this town
//	  start_ms: 13170
//	  end_ms: 16520
//	plain: |
//	  He said, "Let's get out of this town
//
// It returns nil when there is nothing to describe, so the field marshals as
// JSON null exactly as lrclib.net does for a record without lyrics.
func BuildLyricsFile(trackName, artistName, albumName string, duration float64, instrumental bool, syncedLyrics, plainLyrics string) *string {
	if strings.TrimSpace(syncedLyrics) == "" && strings.TrimSpace(plainLyrics) == "" {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "version: '%s'\n", LyricsFileVersion)
	b.WriteString("metadata:\n")
	fmt.Fprintf(&b, "  title: %s\n", yamlScalar(trackName))
	fmt.Fprintf(&b, "  artist: %s\n", yamlScalar(artistName))
	fmt.Fprintf(&b, "  album: %s\n", yamlScalar(albumName))
	fmt.Fprintf(&b, "  duration_ms: %d\n", durationMs(duration))
	fmt.Fprintf(&b, "  instrumental: %t\n", instrumental)

	// Each line's end_ms is the next line's start_ms; the final line is left
	// open rather than inventing a duration we do not know.
	lines := parseTimedLines(syncedLyrics)
	if len(lines) > 0 {
		b.WriteString("lines:\n")
		for i, line := range lines {
			fmt.Fprintf(&b, "- text: %s\n", yamlScalar(line.text))
			fmt.Fprintf(&b, "  start_ms: %d\n", line.startMs)
			if i+1 < len(lines) {
				fmt.Fprintf(&b, "  end_ms: %d\n", lines[i+1].startMs)
			}
		}
	}

	if plainLyrics == "" {
		b.WriteString("plain: ''\n")
	} else {
		b.WriteString("plain: |\n")
		for _, line := range strings.Split(strings.TrimRight(plainLyrics, "\n"), "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	out := b.String()
	return &out
}

// parseTimedLines extracts (start_ms, text) pairs from LRC content, expands
// lines carrying several timestamps, and sorts by start time. Metadata
// headers such as [ti:] or [ar:] are skipped.
func parseTimedLines(synced string) []timedLine {
	var out []timedLine

	for _, raw := range strings.Split(synced, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || headerTagRegex.MatchString(line) {
			continue
		}

		tags := lrcTimeTag.FindAllStringSubmatch(line, -1)
		if len(tags) == 0 {
			continue
		}
		text := strings.TrimSpace(lrcTimeTag.ReplaceAllString(line, ""))

		for _, tag := range tags {
			out = append(out, timedLine{startMs: lrcTagToMs(tag), text: text})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].startMs < out[j].startMs })
	return out
}

// lrcTagToMs converts a matched [mm:ss.fff] tag into milliseconds.
func lrcTagToMs(tag []string) int {
	minutes, _ := strconv.Atoi(tag[1])
	seconds, _ := strconv.Atoi(tag[2])

	var millis int
	if tag[3] != "" {
		frac := tag[3]
		switch len(frac) {
		case 1:
			frac += "00"
		case 2:
			frac += "0"
		}
		millis, _ = strconv.Atoi(frac)
	}

	return (minutes*60+seconds)*1000 + millis
}

// durationMs converts a seconds value to whole milliseconds.
func durationMs(seconds float64) int {
	if seconds <= 0 {
		return 0
	}
	return int(seconds*1000 + 0.5)
}

// yamlScalar renders a value as a YAML scalar, quoting only when the raw form
// would be ambiguous. Lyrics routinely contain ": " ("作词 : 方文山") and
// leading punctuation, both of which would otherwise change the document's
// meaning or make it unparseable.
func yamlScalar(s string) string {
	// A timed line is never multi-line; keep the document structurally simple.
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)

	if s == "" {
		return "''"
	}
	if yamlNeedsQuote(s) {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return s
}

func yamlNeedsQuote(s string) bool {
	if s != strings.TrimSpace(s) {
		return true
	}
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") || strings.Contains(s, " #") {
		return true
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "~", "yes", "no", "on", "off":
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	switch s[0] {
	case '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
		return true
	}
	return false
}
