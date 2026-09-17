// Package model holds the shared shapes: the LRCLIB response, the provider
// candidate, and the incoming query.

package model

import "fmt"

// LyricResponse is an LRCLIB-compatible lyrics record.
//
// The field set and the JSON null semantics mirror lrclib.net as verified
// against the live service:
//
//   - `name` is always present (never omitted).
//   - plainLyrics / syncedLyrics / lyricsfile are emitted as JSON null when
//     absent; LRCLIB never sends an empty string for those.
//
// Field order matters too: it determines the marshalled byte order, which
// callers comparing bodies byte-for-byte will notice.
type LyricResponse struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  *string `json:"plainLyrics"`
	SyncedLyrics *string `json:"syncedLyrics"`
	LyricsFile   *string `json:"lyricsfile"`
}

// ErrorResponse is LRCLIB's error envelope. lrclib.net answers a failed
// /api/get with exactly:
//
//	{"message":"Failed to find specified track","name":"TrackNotFound","statusCode":404}
type ErrorResponse struct {
	Message    string `json:"message"`
	Name       string `json:"name"`
	StatusCode int    `json:"statusCode"`
}

// Error names used by the LRCLIB API.
const (
	ErrNameTrackNotFound    = "TrackNotFound"
	ErrNameServerOverloaded = "ServerOverloaded"
)

// TrackNotFoundError is the canonical 404 body emitted by lrclib.net when no
// record satisfies the request.
func TrackNotFoundError() ErrorResponse {
	return ErrorResponse{
		Message:    "Failed to find specified track",
		Name:       ErrNameTrackNotFound,
		StatusCode: 404,
	}
}

// ServerOverloadedError mirrors the 503 lrclib.net returns when it is busy.
func ServerOverloadedError() ErrorResponse {
	return ErrorResponse{
		Message:    "The server is busy, please retry in a moment",
		Name:       ErrNameServerOverloaded,
		StatusCode: 503,
	}
}

// MissingFieldError renders LRCLIB's plain-text 400 body. lrclib.net surfaces
// the underlying serde deserialization failure verbatim rather than JSON.
func MissingFieldError(field string) string {
	return fmt.Sprintf("Failed to deserialize query string: missing field `%s`", field)
}

// StrPtr maps an empty string to nil so it marshals as JSON null. LRCLIB
// distinguishes "no lyrics" (null) from an empty lyric body.
func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StrVal safely dereferences a nullable string.
func StrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
