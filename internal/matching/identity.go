package matching

import (
	"fmt"
	"math"
)

// DurationBucketSeconds is the width of the duration bucket that tells one
// edit of a track from another.
//
// It must stay strictly narrower than DurationWindowSeconds, otherwise two
// requests landing in the same bucket could be re-validated apart and a cache
// hit would be rejected in a loop. At 2s it sits inside the ±3s window.
const DurationBucketSeconds = 2.0

// DurationBucket maps a duration onto its bucket. An unknown (non-positive)
// duration gets its own bucket, distinct from every real duration, so a
// duration-less request never shares a row with a timed one.
func DurationBucket(duration float64) int {
	if duration <= 0 {
		return -1
	}
	return int(math.Round(duration / DurationBucketSeconds))
}

// IdentityKey identifies a specific edit of a track: the title and artist
// normalized for identity, plus the duration bucket.
//
// This single definition is used for BOTH the storage key and result
// deduplication, which is what keeps them consistent. When they were computed
// separately (a 2s bucket for storage, whole seconds for dedupe) two
// candidates could collapse onto one cache row and then be reported as two
// results carrying the same row id.
func IdentityKey(trackName, artistName string, duration float64) string {
	return fmt.Sprintf("%s\x00%s\x00%d",
		NormalizeIdentity(trackName),
		NormalizeIdentity(artistName),
		DurationBucket(duration),
	)
}
