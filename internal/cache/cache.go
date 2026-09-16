package cache

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/scoring"
)

// Store defines cache storage operations
type Store interface {
	Get(cacheKey string) (*model.LyricResponse, error)
	GetByID(id int) (*model.LyricResponse, error)
	Set(cacheKey string, item *model.LyricResponse) (int, error)
	Close() error
}

// GenerateCacheKey computes a deterministic hash key from title and artist
func GenerateCacheKey(title, artist string) string {
	normTitle := scoring.NormalizeText(title)
	normArtist := scoring.NormalizeText(artist)
	raw := normTitle + "_" + normArtist
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:16]) // 32 hex chars
}
