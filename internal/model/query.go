package model

// Query is a track identification request as it arrives from the LRCLIB
// API layer.
//
// Semantics follow lrclib.net: TrackName and ArtistName are required for
// /api/get, while AlbumName and Duration are optional *filters* — when
// supplied they constrain the match, and when omitted they impose nothing.
type Query struct {
	TrackName  string
	ArtistName string
	AlbumName  string
	Duration   float64
}
