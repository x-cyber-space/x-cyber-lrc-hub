package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/scoring"
)

// Store defines cache storage operations.
type Store interface {
	Get(cacheKey string) (*model.LyricResponse, error)
	GetByID(id int) (*model.LyricResponse, error)
	Set(cacheKey string, item *model.LyricResponse) (int, error)
	// PruneExpired deletes rows past the configured TTL and reports how many
	// were removed. Reads already ignore expired rows, so this only reclaims
	// disk.
	PruneExpired() (int64, error)
	Close() error
}

// GenerateCacheKey computes a deterministic key from a track's identity.
//
// Identity here is deliberately NOT scoring.NormalizeText: that function is
// tuned to make scoring forgiving (it erases "(Live)" so a live take still
// scores against the studio version), and reusing it as a storage key is what
// collapsed different edits of a song onto a single row.
//
// The shape of the identity — title, artist and duration bucket — lives in
// scoring.IdentityKey so that the dispatcher's deduplication and this storage
// key can never drift apart. Only the hashing is cache-specific.
func GenerateCacheKey(title, artist string, duration float64) string {
	sum := sha256.Sum256([]byte(scoring.IdentityKey(title, artist, duration)))
	return hex.EncodeToString(sum[:16]) // 32 hex chars
}

// Matches reports whether a cached record still satisfies the request that
// produced its key.
//
// Both the warm and the cold path run this same predicate, which is the
// property that matters: a fuzzy /api/search result written into the cache
// can never resurface as a confident 404-avoiding /api/get answer, because the
// hit is re-matched against the caller's actual query before it is served.
func Matches(q model.Query, item *model.LyricResponse) bool {
	if item == nil {
		return false
	}
	return scoring.MatchTrack(q, item.TrackName, item.ArtistName, item.AlbumName, item.Duration) != scoring.MatchNone
}

// clampTTL normalizes a TTL for storage: anything non-positive means "never
// expire", which the store represents as zero.
func clampTTL(ttl time.Duration) time.Duration {
	if ttl < 0 {
		return 0
	}
	return ttl
}
