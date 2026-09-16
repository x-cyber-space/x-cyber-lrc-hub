package scoring

import (
	"regexp"
	"strings"
)

// artistSeparator matches the joiners streaming platforms use in credits:
// "A / B", "A & B", "A、B", "A feat. B", "A with B" and friends.
//
// Both sides of a comparison are split the same way, so an artist whose name
// legitimately contains a separator still compares equal to itself — the
// resulting sets are identical. The split only *adds* tolerance, which is why
// it lives in the relaxed stage rather than the exact one.
var artistSeparator = regexp.MustCompile(
	`(?i)\s*(?:/|&|、|;|；|,|，|·|•|\bfeat\.?|\bft\.?|\bfeaturing\b|\bwith\b)\s*`)

// SplitArtists splits a credit string into the set of individual artists it
// names, normalized for identity comparison. Order is preserved and
// duplicates are dropped.
func SplitArtists(s string) []string {
	parts := artistSeparator.Split(strings.ToLower(s), -1)
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		name := CollapseSpaces(bracketRegex.ReplaceAllString(p, " "))
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// ArtistsOverlap reports whether two credit strings share at least one
// artist. This is what lets a request for "周杰伦" match the platform's
// "周杰伦 / 费玉清" credit, or "Beyond" match "Beyond feat. 黄家驹".
func ArtistsOverlap(a, b string) bool {
	left := SplitArtists(a)
	if len(left) == 0 {
		return false
	}
	right := make(map[string]struct{})
	for _, name := range SplitArtists(b) {
		right[name] = struct{}{}
	}
	for _, name := range left {
		if _, ok := right[name]; ok {
			return true
		}
	}
	return false
}
