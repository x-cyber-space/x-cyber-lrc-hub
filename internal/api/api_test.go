package api

import (
	"context"
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

// apiStubProvider is an offline Provider double so the API tests never touch
// the network.
type apiStubProvider struct {
	cands  []*model.Candidate
	lyrics string
}

func (s *apiStubProvider) Name() string { return "stub" }

func (s *apiStubProvider) Search(_ context.Context, _ string, _ int) ([]*model.Candidate, error) {
	out := make([]*model.Candidate, 0, len(s.cands))
	for _, c := range s.cands {
		clone := *c
		clone.Source = "stub"
		out = append(out, &clone)
	}
	return out, nil
}

func (s *apiStubProvider) FetchLyrics(_ context.Context, c *model.Candidate) error {
	if s.lyrics == "" {
		return nil
	}
	c.PlainLyrics = s.lyrics
	c.SyncedLyrics = "[00:00.00] " + s.lyrics
	return nil
}

func setupTestRouter(t *testing.T, cands ...*model.Candidate) (http.Handler, cache.Store) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// ttl 0 disables expiry: these tests are about matching, not freshness.
	store, err := cache.NewSQLiteStore(filepath.Join(tempDir, "test.db"), 0)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return NewRouter(store, provider.NewDispatcherWith(&apiStubProvider{cands: cands}), "test"), store
}

func TestHealthEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
}

// TestGetMissingParamsMatchesLRCLIB pins the plain-text 400 that lrclib.net
// returns for a missing required parameter, byte for byte.
func TestGetMissingParamsMatchesLRCLIB(t *testing.T) {
	router, _ := setupTestRouter(t)

	cases := []struct {
		url  string
		body string
	}{
		{
			url:  "/api/get",
			body: "Failed to deserialize query string: missing field `track_name`",
		},
		{
			url:  "/api/get?track_name=Hurt",
			body: "Failed to deserialize query string: missing field `artist_name`",
		},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.url, nil))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", c.url, rec.Code)
		}
		if got := rec.Body.String(); got != c.body {
			t.Errorf("%s:\n got %q\nwant %q", c.url, got, c.body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("%s: content-type = %q", c.url, ct)
		}
	}
}

// TestGetNotFoundMatchesLRCLIB pins the exact 404 envelope, which is the one
// error body clients actually parse.
func TestGetNotFoundMatchesLRCLIB(t *testing.T) {
	// No candidates at all, so the cold path legitimately misses.
	router, _ := setupTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/get?track_name=Nope&artist_name=Nobody", nil))

	const want = `{"message":"Failed to find specified track","name":"TrackNotFound","statusCode":404}`
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != want {
		t.Fatalf("404 body mismatch:\n got %s\nwant %s", got, want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

func TestGetMethodNotAllowedIsEmpty(t *testing.T) {
	router, _ := setupTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/get", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("lrclib returns an empty 405 body, got %q", body)
	}
}

func TestUnknownRouteIsEmpty404(t *testing.T) {
	router, _ := setupTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("expected an empty 404 body, got %q", body)
	}
}

func TestSearchEmptyQueryReturnsEmptyArray(t *testing.T) {
	router, _ := setupTestRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/search", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "[]" {
		t.Fatalf("expected [], got %q", body)
	}
}

// TestCacheHitShapeMatchesColdShape covers the response-shape bug where the
// cold path omitted `name` while the warm path included it, making the wire
// format depend on cache state.
func TestCacheHitShapeMatchesColdShape(t *testing.T) {
	router, store := setupTestRouter(t)

	key := cache.GenerateCacheKey("七里香", "周杰伦", 299)
	if _, err := store.Set(key, &model.LyricResponse{
		Name:         "七里香",
		TrackName:    "七里香",
		ArtistName:   "周杰伦",
		AlbumName:    "七里香",
		Duration:     299,
		PlainLyrics:  model.StrPtr("窗外的麻雀"),
		SyncedLyrics: model.StrPtr("[00:18.50]窗外的麻雀"),
		LyricsFile:   nil,
	}); err != nil {
		t.Fatalf("Failed to preload cache: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/get?track_name=七里香&artist_name=周杰伦&duration=299", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Every field lrclib.net emits must be present on the warm path too.
	for _, field := range []string{
		"id", "name", "trackName", "artistName", "albumName",
		"duration", "instrumental", "plainLyrics", "syncedLyrics", "lyricsfile",
	} {
		if _, ok := raw[field]; !ok {
			t.Errorf("warm response is missing field %q", field)
		}
	}

	// A record cached without lyricsfile must still emit the key, as null.
	if got := string(raw["lyricsfile"]); got != "null" {
		t.Errorf("lyricsfile = %s, want null", got)
	}
}

// TestCachedRowOutsideQueryIsNotServed closes the cache-poisoning path: a row
// sitting in the cache that does not satisfy the request must be ignored.
func TestCachedRowOutsideQueryIsNotServed(t *testing.T) {
	router, store := setupTestRouter(t)

	key := cache.GenerateCacheKey("童话", "光良", 246)
	if _, err := store.Set(key, &model.LyricResponse{
		Name: "童话", TrackName: "童话", ArtistName: "王菲", Duration: 255,
		PlainLyrics: model.StrPtr("错歌的歌词"),
	}); err != nil {
		t.Fatalf("Failed to preload cache: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/get?track_name=童话&artist_name=光良&duration=246", nil))

	// The dispatcher has no candidates, so a correct implementation misses.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a cached row by a different artist must not be served, got %d: %s", rec.Code, rec.Body.String())
	}
}
