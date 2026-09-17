package matching

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// bracketPatterns matches content within various brackets: (..), [..], （..）, 【..】, {..}
	bracketRegex = regexp.MustCompile(`(?i)\([^)]*\)|\[[^\]]*\]|（[^）]*）|【[^】]*】|{[^}]*}`)
)

// NormalizeIdentity normalizes a field for *identity* comparison, mirroring
// LRCLIB's matching rules as measured against the live service: lowercase,
// strip bracketed modifiers, and collapse whitespace.
//
//	"Wildest Dreams"                  -> "wildest dreams"
//	"Wildest  Dreams"                 -> "wildest dreams"   (double space)
//	"Wildest Dreams (Taylor's Version)" -> "wildest dreams" (bracket stripped)
//	"Wildest Dream"                   -> "wildest dream"    (no fuzzy matching)
//
// Punctuation is deliberately preserved, so this is stricter than
// NormalizeText. The two must stay separate: NormalizeText exists to make
// *scoring* forgiving, NormalizeIdentity exists to make *matching* exact.
// Conflating them is what let "(Live)" and the studio take collapse onto one
// cache row.
func NormalizeIdentity(s string) string {
	s = strings.ToLower(s)
	s = bracketRegex.ReplaceAllString(s, " ")
	return CollapseSpaces(s)
}

// CollapseSpaces trims and collapses every run of whitespace to one space.
func CollapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

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
