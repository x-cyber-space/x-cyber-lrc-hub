package scoring

// BigramJaccard calculates similarity between two strings using bigrams.
// Returns a value between 0.0 and 1.0.
func BigramJaccard(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	runes1 := []rune(s1)
	runes2 := []rune(s2)

	// If very short strings (< 2 runes), compare directly or by character overlap
	if len(runes1) < 2 || len(runes2) < 2 {
		return charJaccard(runes1, runes2)
	}

	set1 := make(map[string]struct{})
	for i := 0; i < len(runes1)-1; i++ {
		set1[string(runes1[i:i+2])] = struct{}{}
	}

	set2 := make(map[string]struct{})
	for i := 0; i < len(runes2)-1; i++ {
		set2[string(runes2[i:i+2])] = struct{}{}
	}

	intersection := 0
	for k := range set1 {
		if _, ok := set2[k]; ok {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func charJaccard(r1, r2 []rune) float64 {
	set1 := make(map[rune]struct{})
	for _, r := range r1 {
		set1[r] = struct{}{}
	}

	set2 := make(map[rune]struct{})
	for _, r := range r2 {
		set2[r] = struct{}{}
	}

	intersection := 0
	for r := range set1 {
		if _, ok := set2[r]; ok {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
