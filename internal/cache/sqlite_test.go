package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

func TestSQLiteStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lrchub_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "sub", "lyrics.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create sqlite store: %v", err)
	}
	defer store.Close()

	key := GenerateCacheKey("七里香", "周杰伦")
	if key == "" {
		t.Fatalf("Key generation failed")
	}

	// 1. Initial Get should return nil
	item, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item != nil {
		t.Fatalf("Expected nil item for miss, got %+v", item)
	}

	// 2. Set item
	toSet := &model.LyricResponse{
		TrackName:    "七里香",
		ArtistName:   "周杰伦",
		AlbumName:    "七里香",
		Duration:     299.0,
		Instrumental: false,
		PlainLyrics:  "窗外的麻雀 在电线杆上多嘴",
		SyncedLyrics: "[00:18.50]窗外的麻雀 在电线杆上多嘴",
	}

	id, err := store.Set(key, toSet)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Expected valid id, got %d", id)
	}

	// 3. Get item by key
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get by key failed: %v", err)
	}
	if got == nil || got.TrackName != "七里香" || got.ID != id {
		t.Fatalf("Unexpected got: %+v", got)
	}

	// 4. Get item by ID
	gotByID, err := store.GetByID(id)
	if err != nil {
		t.Fatalf("Get by ID failed: %v", err)
	}
	if gotByID == nil || gotByID.TrackName != "七里香" {
		t.Fatalf("Unexpected gotByID: %+v", gotByID)
	}
}
