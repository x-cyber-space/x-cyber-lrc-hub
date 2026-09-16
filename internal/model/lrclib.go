package model

// LyricResponse represents the standard LRCLIB API response object
type LyricResponse struct {
	ID           int     `json:"id"`
	Name         string  `json:"name,omitempty"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// ErrorResponse represents the LRCLIB standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}
