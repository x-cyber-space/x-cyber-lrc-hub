package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// SQLiteStore implements Store using pure-Go SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database at the specified path
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Configure connection pool for SQLite single-writer safety
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) initTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS lyrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT UNIQUE NOT NULL,
		track_name TEXT NOT NULL,
		artist_name TEXT NOT NULL,
		album_name TEXT,
		duration REAL,
		instrumental INTEGER DEFAULT 0,
		plain_lyrics TEXT,
		synced_lyrics TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_lyrics_cache_key ON lyrics(cache_key);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite tables: %w", err)
	}
	return nil
}

// Get retrieves cached lyric response by cacheKey
func (s *SQLiteStore) Get(cacheKey string) (*model.LyricResponse, error) {
	query := `
	SELECT id, track_name, artist_name, album_name, duration, instrumental, plain_lyrics, synced_lyrics
	FROM lyrics
	WHERE cache_key = ?
	LIMIT 1;
	`
	var (
		id           int
		trackName    string
		artistName   string
		albumName    sql.NullString
		duration     float64
		instrumental int
		plainLyrics  sql.NullString
		syncedLyrics sql.NullString
	)

	err := s.db.QueryRow(query, cacheKey).Scan(
		&id,
		&trackName,
		&artistName,
		&albumName,
		&duration,
		&instrumental,
		&plainLyrics,
		&syncedLyrics,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // cache miss
		}
		return nil, err
	}

	return &model.LyricResponse{
		ID:           id,
		Name:         trackName,
		TrackName:    trackName,
		ArtistName:   artistName,
		AlbumName:    albumName.String,
		Duration:     duration,
		Instrumental: instrumental == 1,
		PlainLyrics:  plainLyrics.String,
		SyncedLyrics: syncedLyrics.String,
	}, nil
}

// GetByID retrieves cached lyric response by integer ID
func (s *SQLiteStore) GetByID(id int) (*model.LyricResponse, error) {
	query := `
	SELECT id, track_name, artist_name, album_name, duration, instrumental, plain_lyrics, synced_lyrics
	FROM lyrics
	WHERE id = ?
	LIMIT 1;
	`
	var (
		dbID         int
		trackName    string
		artistName   string
		albumName    sql.NullString
		duration     float64
		instrumental int
		plainLyrics  sql.NullString
		syncedLyrics sql.NullString
	)

	err := s.db.QueryRow(query, id).Scan(
		&dbID,
		&trackName,
		&artistName,
		&albumName,
		&duration,
		&instrumental,
		&plainLyrics,
		&syncedLyrics,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &model.LyricResponse{
		ID:           dbID,
		Name:         trackName,
		TrackName:    trackName,
		ArtistName:   artistName,
		AlbumName:    albumName.String,
		Duration:     duration,
		Instrumental: instrumental == 1,
		PlainLyrics:  plainLyrics.String,
		SyncedLyrics: syncedLyrics.String,
	}, nil
}

// Set stores or updates lyrics in SQLite and returns the record ID
func (s *SQLiteStore) Set(cacheKey string, item *model.LyricResponse) (int, error) {
	instr := 0
	if item.Instrumental {
		instr = 1
	}

	query := `
	INSERT INTO lyrics (cache_key, track_name, artist_name, album_name, duration, instrumental, plain_lyrics, synced_lyrics, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(cache_key) DO UPDATE SET
		track_name=excluded.track_name,
		artist_name=excluded.artist_name,
		album_name=excluded.album_name,
		duration=excluded.duration,
		instrumental=excluded.instrumental,
		plain_lyrics=excluded.plain_lyrics,
		synced_lyrics=excluded.synced_lyrics,
		updated_at=CURRENT_TIMESTAMP;
	`
	_, err := s.db.Exec(query,
		cacheKey,
		item.TrackName,
		item.ArtistName,
		item.AlbumName,
		item.Duration,
		instr,
		item.PlainLyrics,
		item.SyncedLyrics,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert/update lyric cache: %w", err)
	}

	// Fetch current ID
	var id int
	err = s.db.QueryRow("SELECT id FROM lyrics WHERE cache_key = ?", cacheKey).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch id after insert: %w", err)
	}

	item.ID = id
	return id, nil
}

// Close closes the underlying SQLite database
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
