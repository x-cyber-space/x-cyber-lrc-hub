package scoring

import (
	"math"
	"strings"
)

// CalculateScore computes the fitness score (0-100) between target song metadata and candidate metadata
func CalculateScore(targetTitle, targetArtist string, targetDuration float64,
	candTitle, candArtist string, candDuration float64) float64 {

	titleScore := calculateTitleScore(targetTitle, candTitle)
	artistScore := calculateArtistScore(targetArtist, candArtist)
	durationScore := calculateDurationScore(targetDuration, candDuration)

	total := titleScore + artistScore + durationScore
	if total > 100.0 {
		total = 100.0
	}
	return math.Round(total*100) / 100
}

func calculateTitleScore(target, cand string) float64 {
	normTarget := NormalizeText(target)
	normCand := NormalizeText(cand)

	if normTarget == "" || normCand == "" {
		return 0.0
	}

	if normTarget == normCand {
		return 45.0
	}

	if strings.Contains(normTarget, normCand) || strings.Contains(normCand, normTarget) {
		return 38.0
	}

	sim := BigramJaccard(normTarget, normCand)
	return sim * 45.0
}

func calculateArtistScore(target, cand string) float64 {
	if IsUnknownArtist(target) {
		return 20.0
	}

	normTarget := NormalizeText(target)
	normCand := NormalizeText(cand)

	if normTarget == "" || normCand == "" {
		return 10.0
	}

	if normTarget == normCand {
		return 30.0
	}

	if strings.Contains(normTarget, normCand) || strings.Contains(normCand, normTarget) {
		return 28.0
	}

	sim := BigramJaccard(normTarget, normCand)
	return sim * 30.0
}

func calculateDurationScore(target, cand float64) float64 {
	// If caller didn't provide target duration, return a neutral score
	if target <= 0 || cand <= 0 {
		return 20.0
	}

	diff := math.Abs(target - cand)
	switch {
	case diff < 1.0:
		return 25.0
	case diff < 2.0:
		return 24.0
	case diff < 3.0:
		return 22.0
	case diff < 5.0:
		return 15.0
	default:
		return 0.0
	}
}
