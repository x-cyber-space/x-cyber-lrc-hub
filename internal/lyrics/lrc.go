// Package lyrics parses and renders LRC: plain-text extraction, instrumental
// detection, and the lyricsfile document lrclib.net serves alongside them.
//
// It works on neutral types only. Platform-specific wire formats belong to the
// provider that speaks them.

package lyrics

import (
	"regexp"
	"strings"
)

var (
	// timeTagRegex matches LRC timestamps like [00:12.34] or [00:12.345] or [00:12]
	timeTagRegex = regexp.MustCompile(`\[\d{1,2}:\d{1,2}(?:\.\d{1,3})?\]`)
	// headerTagRegex matches metadata tags like [ar:xxx], [ti:xxx], [al:xxx], [by:xxx], etc.
	headerTagRegex = regexp.MustCompile(`^\[[a-zA-Z]+:[^\]]*\]`)
)

// ExtractPlainLyrics extracts plain text lyrics from synced LRC lyrics by removing timestamps and headers
func ExtractPlainLyrics(syncedLyrics string) string {
	if syncedLyrics == "" {
		return ""
	}

	lines := strings.Split(syncedLyrics, "\n")
	var plainLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip metadata headers like [ti:Title], [ar:Artist]
		if headerTagRegex.MatchString(trimmed) {
			continue
		}

		// Remove all time tags
		cleanLine := strings.TrimSpace(timeTagRegex.ReplaceAllString(trimmed, ""))
		if cleanLine != "" {
			plainLines = append(plainLines, cleanLine)
		}
	}

	return strings.Join(plainLines, "\n")
}

// IsInstrumental detects whether the lyrics represent an instrumental piece
func IsInstrumental(plainLyrics, syncedLyrics string) bool {
	combined := strings.ToLower(plainLyrics + " " + syncedLyrics)
	patterns := []string{
		"纯音乐",
		"请欣赏",
		"instrumental",
		"没有填词",
		"无歌词",
		"纯音乐，请欣赏",
	}

	for _, p := range patterns {
		if strings.Contains(combined, p) {
			// Check if there are only a couple of lines indicating instrumental
			plainLines := strings.Split(strings.TrimSpace(plainLyrics), "\n")
			if len(plainLines) <= 3 {
				return true
			}
		}
	}

	return false
}
