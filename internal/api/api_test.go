package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

func setupTestRouter(t *testing.T) (http.Handler, cache.Store, func()) {
	tempDir, err := os.MkdirTemp("", "api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := cache.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	dispatcher := provider.NewDispatcher()
	router := NewRouter(store, dispatcher)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return router, store, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
}

func TestGetEndpointMissingParams(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/get", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing params, got %d", rec.Code)
	}
}

func TestGetEndpointCacheHit(t *testing.T) {
	router, store, cleanup := setupTestRouter(t)
	defer cleanup()

	// Preload into cache
	key := cache.GenerateCacheKey("七里香", "周杰伦")
	_, err := store.Set(key, &model.LyricResponse{
		TrackName:    "七里香",
		ArtistName:   "周杰伦",
		AlbumName:    "七里香",
		Duration:     299.0,
		PlainLyrics:  "窗外的麻雀",
		SyncedLyrics: "[00:18.50]窗外的麻雀",
	})
	if err != nil {
		t.Fatalf("Failed to preload cache: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/get?track_name=七里香&artist_name=周杰伦&duration=299", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.LyricResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.TrackName != "七里香" || resp.ID <= 0 {
		t.Fatalf("Unexpected cached response: %+v", resp)
	}
}
