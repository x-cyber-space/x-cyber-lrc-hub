package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// SearchHandler handles GET /api/search
func SearchHandler(store cache.Store, dispatcher *provider.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		query := strings.TrimSpace(q.Get("q"))
		trackName := strings.TrimSpace(q.Get("track_name"))
		artistName := strings.TrimSpace(q.Get("artist_name"))

		if query == "" && trackName == "" && artistName == "" {
			writeJSON(w, http.StatusOK, []*model.LyricResponse{})
			return
		}

		targetTitle := trackName
		if targetTitle == "" {
			targetTitle = query
		}

		log.Printf("[search-list] query=%q, track=%q, artist=%q", query, trackName, artistName)

		_, validList, err := dispatcher.SearchAndScore(r.Context(), targetTitle, artistName, "", 0, 30.0)
		if err != nil {
			log.Printf("[search-list-error] %v", err)
			writeJSON(w, http.StatusOK, []*model.LyricResponse{})
			return
		}

		var results []*model.LyricResponse
		for _, item := range validList {
			cacheKey := cache.GenerateCacheKey(item.TrackName, item.ArtistName)
			respItem := &model.LyricResponse{
				TrackName:    item.TrackName,
				ArtistName:   item.ArtistName,
				AlbumName:    item.AlbumName,
				Duration:     item.Duration,
				Instrumental: item.Instrumental,
				PlainLyrics:  item.PlainLyrics,
				SyncedLyrics: item.SyncedLyrics,
			}

			// Save to cache or lookup ID
			id, err := store.Set(cacheKey, respItem)
			if err == nil {
				respItem.ID = id
			}
			results = append(results, respItem)
		}

		if results == nil {
			results = []*model.LyricResponse{}
		}

		writeJSON(w, http.StatusOK, results)
	}
}
