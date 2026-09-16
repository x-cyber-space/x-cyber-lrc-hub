package model

// Candidate represents a candidate song matched from one of the music providers
type Candidate struct {
	Source       string  `json:"source"`
	SongID       string  `json:"songId"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"` // Duration in seconds
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
	Score        float64 `json:"score"`
}
