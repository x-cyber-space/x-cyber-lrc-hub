package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/scoring"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	return newTestStoreWithTTL(t, 0)
}

func newTestStoreWithTTL(t *testing.T, ttl time.Duration) *SQLiteStore {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lrchub_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	store, err := NewSQLiteStore(filepath.Join(tempDir, "sub", "lyrics.db"), ttl)
	if err != nil {
		t.Fatalf("Failed to create sqlite store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return store
}

func TestSQLiteStoreRoundTrip(t *testing.T) {
	store := newTestStore(t)

	key := GenerateCacheKey("七里香", "周杰伦", 299)
	if key == "" {
		t.Fatal("Key generation failed")
	}

	// 1. Initial Get should miss.
	item, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item != nil {
		t.Fatalf("Expected nil item for miss, got %+v", item)
	}

	// 2. Set.
	toSet := &model.LyricResponse{
		Name:         "七里香",
		TrackName:    "七里香",
		ArtistName:   "周杰伦",
		AlbumName:    "七里香",
		Duration:     299,
		PlainLyrics:  model.StrPtr("窗外的麻雀 在电线杆上多嘴"),
		SyncedLyrics: model.StrPtr("[00:18.50]窗外的麻雀 在电线杆上多嘴"),
		LyricsFile:   model.StrPtr("version: '1.0'\n"),
	}
	id, err := store.Set(key, toSet)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Expected valid id, got %d", id)
	}

	// 3. Get by key.
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get by key failed: %v", err)
	}
	if got == nil || got.TrackName != "七里香" || got.ID != id {
		t.Fatalf("Unexpected got: %+v", got)
	}
	if model.StrVal(got.SyncedLyrics) != "[00:18.50]窗外的麻雀 在电线杆上多嘴" {
		t.Fatalf("synced lyrics did not round-trip: %q", model.StrVal(got.SyncedLyrics))
	}
	if model.StrVal(got.LyricsFile) != "version: '1.0'\n" {
		t.Fatalf("lyricsfile did not round-trip: %q", model.StrVal(got.LyricsFile))
	}

	// 4. Get by ID.
	gotByID, err := store.GetByID(id)
	if err != nil {
		t.Fatalf("Get by ID failed: %v", err)
	}
	if gotByID == nil || gotByID.TrackName != "七里香" {
		t.Fatalf("Unexpected gotByID: %+v", gotByID)
	}
}

// TestSQLiteStorePreservesNullLyrics pins the null/empty distinction: LRCLIB
// sends null for absent lyrics, and a round-trip through a TEXT column must
// not quietly turn that into "".
func TestSQLiteStorePreservesNullLyrics(t *testing.T) {
	store := newTestStore(t)

	key := GenerateCacheKey("Instrumental Piece", "Someone", 150)
	if _, err := store.Set(key, &model.LyricResponse{
		Name:         "Instrumental Piece",
		TrackName:    "Instrumental Piece",
		ArtistName:   "Someone",
		Duration:     150,
		Instrumental: true,
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := store.Get(key)
	if err != nil || got == nil {
		t.Fatalf("Get failed: %v (%+v)", err, got)
	}
	if got.PlainLyrics != nil || got.SyncedLyrics != nil || got.LyricsFile != nil {
		t.Fatalf("absent lyrics must stay nil so they marshal as null, got %+v", got)
	}
	if !got.Instrumental {
		t.Fatal("instrumental flag did not round-trip")
	}
}

func TestSQLiteStoreUpsertReusesRow(t *testing.T) {
	store := newTestStore(t)

	key := GenerateCacheKey("晴天", "周杰伦", 269)
	first, err := store.Set(key, &model.LyricResponse{
		Name: "晴天", TrackName: "晴天", ArtistName: "周杰伦", Duration: 269,
		PlainLyrics: model.StrPtr("旧歌词"),
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	second, err := store.Set(key, &model.LyricResponse{
		Name: "晴天", TrackName: "晴天", ArtistName: "周杰伦", Duration: 269,
		PlainLyrics: model.StrPtr("新歌词"),
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if first != second {
		t.Fatalf("upsert should reuse the row id, got %d then %d", first, second)
	}

	got, _ := store.Get(key)
	if got == nil || model.StrVal(got.PlainLyrics) != "新歌词" {
		t.Fatalf("upsert did not update the row: %+v", got)
	}
}

// TestGenerateCacheKeyIdentity pins how the storage key relates to the other
// two normalizations in this project.
//
// The edits of a song are told apart by DURATION, not by their bracketed
// modifiers: lrclib.net itself matches "Wildest Dreams (Taylor's Version)" to
// the original record, so "(Live)" and the bare title are one identity as far
// as LRCLIB is concerned. Two forms sharing a duration therefore share a row,
// which is safe because every cache hit is re-validated against the caller's
// query, and a live take almost always differs in length anyway.
func TestGenerateCacheKeyIdentity(t *testing.T) {
	studio := GenerateCacheKey("七里香", "周杰伦", 299)

	// A live take runs to a different length, landing in a different bucket.
	if studio == GenerateCacheKey("七里香 (Live)", "周杰伦", 295) {
		t.Error("edits whose durations differ must not share a cache row")
	}
	if studio == GenerateCacheKey("七里香", "周杰伦", 350) {
		t.Error("edits with clearly different durations must not share a cache row")
	}
	if studio == GenerateCacheKey("晴天", "周杰伦", 299) {
		t.Error("different titles must not share a cache row")
	}

	// ...but a sub-bucket jitter around one edit stays on one row, so ordinary
	// metadata drift does not fragment the cache.
	if studio != GenerateCacheKey("七里香", "周杰伦", 299.4) {
		t.Error("sub-bucket duration jitter should reuse the same row")
	}

	// Bracket modifiers are erased on purpose, matching LRCLIB's own matching.
	if studio != GenerateCacheKey("七里香 (Live)", "周杰伦", 299) {
		t.Error("bracket variants are one identity under LRCLIB matching rules")
	}

	// The bucket must stay narrower than the ±3s matching window, otherwise a
	// cache hit could be re-validated and rejected, causing a refetch loop.
	if scoring.DurationBucketSeconds >= scoring.DurationWindowSeconds {
		t.Fatalf("duration bucket %v is not narrower than the %v match window",
			scoring.DurationBucketSeconds, scoring.DurationWindowSeconds)
	}
}

// TestMatchesRejectsCachedMismatch proves the property that closes the cache
// poisoning hole: a row that does not satisfy the caller's query is treated as
// a miss, so a fuzzy /api/search result can never become an authoritative
// /api/get answer just by sitting in the cache.
func TestMatchesRejectsCachedMismatch(t *testing.T) {
	base := &model.LyricResponse{
		Name: "童话", TrackName: "童话", ArtistName: "王菲", Duration: 255,
		PlainLyrics: model.StrPtr("词"),
	}

	if Matches(model.Query{TrackName: "童话", ArtistName: "光良", Duration: 246}, base) {
		t.Fatal("cached row by a different artist must not satisfy the query")
	}
	if !Matches(model.Query{TrackName: "童话", ArtistName: "王菲", Duration: 255}, base) {
		t.Fatal("cached row must satisfy its own query")
	}
	if Matches(model.Query{TrackName: "童话", ArtistName: "王菲", Duration: 300}, base) {
		t.Fatal("cached row outside the duration window must not satisfy the query")
	}
	if Matches(model.Query{TrackName: "童话", ArtistName: "王菲"}, nil) {
		t.Fatal("a nil row is never a match")
	}
}

// TestStoreExpiresRows pins the freshness contract: an expired row reads as a
// miss, and pruning removes it. Without this, the copy fetched on the very
// first request would be served forever even after a provider corrects it.
func TestStoreExpiresRows(t *testing.T) {
	store := newTestStoreWithTTL(t, 60*time.Millisecond)

	key := GenerateCacheKey("七里香", "周杰伦", 299)
	if _, err := store.Set(key, &model.LyricResponse{
		Name: "七里香", TrackName: "七里香", ArtistName: "周杰伦", Duration: 299,
		PlainLyrics: model.StrPtr("词"),
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Fresh: readable.
	got, err := store.Get(key)
	if err != nil || got == nil {
		t.Fatalf("a freshly written row must be readable (err=%v, got=%+v)", err, got)
	}

	time.Sleep(120 * time.Millisecond)

	// Expired: reads as a miss even before pruning runs.
	got, err = store.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Fatal("an expired row must read as a miss")
	}

	// And pruning reclaims it.
	removed, err := store.PruneExpired()
	if err != nil {
		t.Fatalf("PruneExpired failed: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 row pruned, got %d", removed)
	}

	// Idempotent.
	removed, err = store.PruneExpired()
	if err != nil {
		t.Fatalf("PruneExpired failed: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected nothing left to prune, got %d", removed)
	}
}

// TestStoreWithoutTTLKeepsRowsForever covers the opt-out.
func TestStoreWithoutTTLKeepsRowsForever(t *testing.T) {
	store := newTestStoreWithTTL(t, 0)

	key := GenerateCacheKey("七里香", "周杰伦", 299)
	if _, err := store.Set(key, &model.LyricResponse{
		Name: "七里香", TrackName: "七里香", ArtistName: "周杰伦", Duration: 299,
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	removed, err := store.PruneExpired()
	if err != nil {
		t.Fatalf("PruneExpired failed: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expiry is disabled, nothing should be pruned (got %d)", removed)
	}

	got, err := store.Get(key)
	if err != nil || got == nil {
		t.Fatalf("row must survive with expiry disabled (err=%v, got=%+v)", err, got)
	}
	if store.TTL() != 0 {
		t.Fatalf("TTL() = %v, want 0", store.TTL())
	}
}

// TestNegativeTTLIsTreatedAsDisabled keeps a nonsensical config from turning
// into "every row is immediately stale".
func TestNegativeTTLIsTreatedAsDisabled(t *testing.T) {
	store := newTestStoreWithTTL(t, -time.Hour)

	key := GenerateCacheKey("七里香", "周杰伦", 299)
	if _, err := store.Set(key, &model.LyricResponse{
		Name: "七里香", TrackName: "七里香", ArtistName: "周杰伦", Duration: 299,
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if got, _ := store.Get(key); got == nil {
		t.Fatal("a negative TTL must behave as 'never expire'")
	}
}

// TestStoreUsesWAL checks the durability pragma actually took effect, since a
// PRAGMA issued once only applies to the connection it ran on and it would be
// easy to lose silently.
func TestStoreUsesWAL(t *testing.T) {
	store := newTestStore(t)

	var mode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("failed to read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}

	var busy int
	if err := store.db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("failed to read busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busy)
	}
}

// TestConcurrentCacheAccess exercises the single-connection pool under -race.
func TestConcurrentCacheAccess(t *testing.T) {
	store := newTestStore(t)

	const writers = 8
	done := make(chan struct{})

	for i := 0; i < writers; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 10; j++ {
				key := GenerateCacheKey("歌", "手", float64(n))
				if _, err := store.Set(key, &model.LyricResponse{
					Name: "歌", TrackName: "歌", ArtistName: "手", Duration: float64(n),
				}); err != nil {
					t.Errorf("Set failed: %v", err)
					return
				}
				if _, err := store.Get(key); err != nil {
					t.Errorf("Get failed: %v", err)
					return
				}
			}
		}(i)
	}

	for i := 0; i < writers; i++ {
		<-done
	}
}
