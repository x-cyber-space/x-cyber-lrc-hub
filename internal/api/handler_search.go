package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// SearchHandler handles GET /api/search.
//
// Mirrors lrclib.net: an empty query set and a resultless query both answer
// `[]` with 200 rather than 404, and results carry their lyrics inline.
func SearchHandler(store cache.Store, dispatcher *provider.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()

		query := strings.TrimSpace(params.Get("q"))
		trackName := strings.TrimSpace(params.Get("track_name"))
		artistName := strings.TrimSpace(params.Get("artist_name"))
		albumName := strings.TrimSpace(params.Get("album_name"))

		if query == "" && trackName == "" && artistName == "" {
			writeJSON(w, http.StatusOK, []*model.LyricResponse{})
			return
		}

		q := model.Query{
			TrackName:  firstNonEmpty(trackName, query),
			ArtistName: artistName,
			AlbumName:  albumName,
		}

		slog.Debug("search requested",
			"q", query, "track", trackName, "artist", artistName, "album", albumName)

		items, err := dispatcher.Search(r.Context(), q)
		if err != nil {
			slog.Error("provider search failed", "query", query, "error", err)
			writeJSON(w, http.StatusOK, []*model.LyricResponse{})
			return
		}

		results := make([]*model.LyricResponse, 0, len(items))
		for _, item := range items {
			resp := newLyricResponse(
				item.TrackName,
				item.ArtistName,
				item.AlbumName,
				item.Duration,
				item.Instrumental,
				item.PlainLyrics,
				item.SyncedLyrics,
			)

			// Cache under the *result's own* identity, not the query's. That
			// is what lets a later /api/get with the same title, artist and
			// duration find it, without different results of one search
			// overwriting each other on a shared key.
			key := cache.GenerateCacheKey(item.TrackName, item.ArtistName, item.Duration)
			if id, err := store.Set(key, resp); err != nil {
				slog.Error("cache write failed", "error", err)
			} else {
				resp.ID = id
			}

			results = append(results, resp)
		}

		slog.Info("search completed", "query", query, "results", len(results))

		writeJSON(w, http.StatusOK, results)
	}
}
