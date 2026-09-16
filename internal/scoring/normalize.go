package scoring

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// bracketPatterns matches content within various brackets: (..), [..], （..）, 【..】, {..}
	bracketRegex = regexp.MustCompile(`(?i)\([^)]*\)|\[[^\]]*\]|（[^）]*）|【[^】]*】|{[^}]*}`)
)

// NormalizeText normalizes song title or artist name by lowercasing,
// removing bracketed modifiers, and removing non-alphanumeric/non-Han characters.
func NormalizeText(s string) string {
	s = strings.ToLower(s)

	// Remove bracketed parts like (Live), [Remix], （伴奏）, 【超清】, etc.
	cleaned := bracketRegex.ReplaceAllString(s, "")
	cleaned = strings.TrimSpace(cleaned)

	// If removing brackets emptied the string completely (e.g. title was "[Instrumental]"), fallback to original
	if cleaned == "" {
		cleaned = s
	}

	// Keep only letters, digits, and Han (Chinese) characters
	var sb strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

// IsUnknownArtist returns true if the artist is unknown or generic
func IsUnknownArtist(artist string) bool {
	norm := NormalizeText(artist)
	if norm == "" {
		return true
	}
	switch norm {
	case "unknown", "unknownartist", "未知歌手", "未知", "群星", "variousartists":
		return true
	}
	return false
}
