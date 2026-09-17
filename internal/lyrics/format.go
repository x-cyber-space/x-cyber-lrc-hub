package lyrics

import (
	"fmt"
	"strings"
)

// TimedLine is one lyric line together with its start offset in seconds.
//
// It is the neutral shape the LRC renderer works on. Each provider is
// responsible for translating its own wire format into this — Kuwo, for
// instance, reports the offset as a string of seconds.
type TimedLine struct {
	Start float64
	Text  string
}

// FormatSynced renders timed lines as standard LRC, returning both the synced
// form and its plain-text projection.
//
// Blank lines are dropped rather than emitted as empty timestamps, and the
// two outputs are kept in lockstep so a caller can rely on line N of each
// describing the same lyric.
func FormatSynced(lines []TimedLine) (syncedLyrics string, plainLyrics string) {
	var synced strings.Builder
	var plain strings.Builder

	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}

		if synced.Len() > 0 {
			synced.WriteString("\n")
			plain.WriteString("\n")
		}

		synced.WriteString(formatTimestamp(line.Start))
		synced.WriteString(" ")
		synced.WriteString(text)

		plain.WriteString(text)
	}

	return synced.String(), plain.String()
}

// formatTimestamp renders a seconds offset as an LRC [mm:ss.xx] tag.
func formatTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := int(seconds) / 60
	return fmt.Sprintf("[%02d:%05.2f]", minutes, seconds-float64(minutes*60))
}
