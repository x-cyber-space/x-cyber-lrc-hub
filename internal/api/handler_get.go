package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// GetHandler handles GET /api/get
func GetHandler(store cache.Store, dispatcher *provider.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		trackName := strings.TrimSpace(q.Get("track_name"))
		artistName := strings.TrimSpace(q.Get("artist_name"))
		albumName := strings.TrimSpace(q.Get("album_name"))
		durationStr := strings.TrimSpace(q.Get("duration"))

		if trackName == "" || artistName == "" {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: "track_name and artist_name are required",
			})
			return
		}

		var duration float64
		if durationStr != "" {
			duration, _ = strconv.ParseFloat(durationStr, 64)
		}

		cacheKey := cache.GenerateCacheKey(trackName, artistName)

		// 1. Try local cache
		cached, err := store.Get(cacheKey)
		if err != nil {
			log.Printf("[cache-error] failed to read cache for %s: %v", cacheKey, err)
		}

		if cached != nil {
			log.Printf("[cache-hit] %s - %s (ID: %d)", cached.TrackName, cached.ArtistName, cached.ID)
			writeJSON(w, http.StatusOK, cached)
			return
		}

		// 2. Cache miss: concurrent fetch from providers
		log.Printf("[search-start] fetching online for: %s - %s (dur: %.1fs)", trackName, artistName, duration)
		best, _, err := dispatcher.SearchAndScore(r.Context(), trackName, artistName, albumName, duration, 40.0)
		if err != nil {
			log.Printf("[search-error] %v", err)
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: "Internal search error",
			})
			return
		}

		if best == nil {
			log.Printf("[search-miss] no lyrics matched for %s - %s", trackName, artistName)
			writeJSON(w, http.StatusNotFound, model.ErrorResponse{
				Error: "Lyrics not found",
			})
			return
		}

		// 3. Construct response and cache result
		album := best.AlbumName
		if album == "" {
			album = albumName
		}

		respItem := &model.LyricResponse{
			TrackName:    best.TrackName,
			ArtistName:   best.ArtistName,
			AlbumName:    album,
			Duration:     best.Duration,
			Instrumental: best.Instrumental,
			PlainLyrics:  best.PlainLyrics,
			SyncedLyrics: best.SyncedLyrics,
		}

		id, err := store.Set(cacheKey, respItem)
		if err != nil {
			log.Printf("[cache-write-error] %v", err)
		} else {
			respItem.ID = id
		}

		log.Printf("[search-hit] %s - %s from %s (score: %.2f, ID: %d)",
			best.TrackName, best.ArtistName, best.Source, best.Score, respItem.ID)

		writeJSON(w, http.StatusOK, respItem)
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
