package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/lyrics"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// GetHandler handles GET /api/get.
//
// Semantics follow lrclib.net: track_name and artist_name are required, while
// album_name and duration act as additional hard filters when supplied (the
// live service returns 404 for an album that does not match, and widens to
// nearest-record within ±3s of the requested duration).
//
// A miss is reported as 404 rather than filled with the best-scoring
// candidate. Scoring is not a gate here: a wrong lyric is indistinguishable
// from a right one at the client, whereas a 404 lets the client retry through
// /api/search with its own ranking and a user in the loop.
func GetHandler(store cache.Store, dispatcher *provider.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()

		q := model.Query{
			TrackName:  strings.TrimSpace(params.Get("track_name")),
			ArtistName: strings.TrimSpace(params.Get("artist_name")),
			AlbumName:  strings.TrimSpace(params.Get("album_name")),
		}

		// Required fields, reported in the order LRCLIB reports them.
		if q.TrackName == "" {
			writeMissingField(w, "track_name")
			return
		}
		if q.ArtistName == "" {
			writeMissingField(w, "artist_name")
			return
		}

		if raw := strings.TrimSpace(params.Get("duration")); raw != "" {
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				slog.Warn("ignoring unparseable duration", "duration", raw, "error", err)
			} else {
				q.Duration = parsed
			}
		}

		key := cache.GenerateCacheKey(q.TrackName, q.ArtistName, q.Duration)

		// 1. Warm path. The row is re-validated with the very same predicate
		// the cold path uses, so a cache hit can never answer a query that a
		// fresh lookup would have rejected — which is how a fuzzy search
		// result used to become an authoritative /api/get answer.
		cached, err := store.Get(key)
		if err != nil {
			slog.Error("cache read failed", "key", key, "error", err)
		}
		if cache.Matches(q, cached) {
			slog.Debug("cache hit",
				"track", cached.TrackName, "artist", cached.ArtistName, "id", cached.ID)
			writeJSON(w, http.StatusOK, cached)
			return
		}

		// 2. Cold path: strict, LRCLIB-compatible matching.
		slog.Debug("cache miss, querying providers",
			"track", q.TrackName, "artist", q.ArtistName,
			"album", q.AlbumName, "duration", q.Duration)

		best, err := dispatcher.Get(r.Context(), q)
		if err != nil {
			slog.Error("provider lookup failed", "track", q.TrackName, "artist", q.ArtistName, "error", err)
			writeJSON(w, http.StatusInternalServerError, model.ServerOverloadedError())
			return
		}
		if best == nil {
			slog.Info("no confident match", "track", q.TrackName, "artist", q.ArtistName)
			writeJSON(w, http.StatusNotFound, model.TrackNotFoundError())
			return
		}

		resp := newLyricResponse(
			best.TrackName,
			best.ArtistName,
			firstNonEmpty(best.AlbumName, q.AlbumName),
			best.Duration,
			best.Instrumental,
			best.PlainLyrics,
			best.SyncedLyrics,
		)

		if id, err := store.Set(key, resp); err != nil {
			slog.Error("cache write failed", "error", err)
		} else {
			resp.ID = id
		}

		slog.Info("match found",
			"track", resp.TrackName, "artist", resp.ArtistName,
			"source", best.Source, "score", best.Score, "id", resp.ID)

		writeJSON(w, http.StatusOK, resp)
	}
}

// newLyricResponse assembles an LRCLIB record. Every field lrclib.net emits is
// populated here — including `name` and `lyricsfile`, which the previous
// implementation either omitted on the cold path or never produced at all,
// leaving the response shape dependent on cache state.
func newLyricResponse(trackName, artistName, albumName string, duration float64, instrumental bool, plainLyrics, syncedLyrics string) *model.LyricResponse {
	return &model.LyricResponse{
		Name:         trackName,
		TrackName:    trackName,
		ArtistName:   artistName,
		AlbumName:    albumName,
		Duration:     duration,
		Instrumental: instrumental,
		PlainLyrics:  model.StrPtr(plainLyrics),
		SyncedLyrics: model.StrPtr(syncedLyrics),
		LyricsFile: lyrics.BuildLyricsFile(
			trackName, artistName, albumName, duration, instrumental, syncedLyrics, plainLyrics,
		),
	}
}
