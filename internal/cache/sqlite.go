package cache

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// schemaVersion is bumped whenever the table layout or the cache key format
// changes. The lyrics table is a pure cache, so a mismatch rebuilds it rather
// than migrating: that also discards rows keyed with the old scheme, which
// could otherwise linger as unreachable data.
//
// v3 stores timestamps with millisecond precision so the TTL comparison does
// not degenerate to whole seconds.
const schemaVersion = 3

// timestampFormat is the SQLite expression used for every timestamp column.
// It must produce the same textual shape that the TTL comparison builds, since
// the comparison is a lexicographic string compare inside SQLite.
const timestampFormat = `strftime('%Y-%m-%d %H:%M:%f','now')`

// SQLiteStore implements Store using pure-Go SQLite.
type SQLiteStore struct {
	db  *sql.DB
	ttl time.Duration
}

// NewSQLiteStore opens or creates a SQLite database at the specified path.
//
// A non-positive ttl disables expiry.
func NewSQLiteStore(dbPath string, ttl time.Duration) (*SQLiteStore, error) {
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

	// One connection, deliberately.
	//
	// SQLite serializes writers regardless, and every query here is an indexed
	// lookup on a small local table — microseconds, against a request whose
	// real cost is four outbound HTTP calls. A larger pool would buy nothing
	// measurable while requiring every PRAGMA below to be re-applied per
	// connection (they are connection-scoped, not database-scoped).
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db, ttl: clampTTL(ttl)}
	if err := store.configure(); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// configure applies durability and locking pragmas.
//
// WAL plus synchronous=NORMAL replaces the default rollback journal's
// per-commit fsyncs with one fsync per checkpoint, which is the difference
// that matters on the spinning disks and SD cards a self-hosted instance
// usually runs on.
func (s *SQLiteStore) configure() error {
	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("failed to enable WAL: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to apply %q: %w", pragma, err)
		}
	}

	return nil
}

func (s *SQLiteStore) initTables() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("failed to read cache schema version: %w", err)
	}
	if version != schemaVersion {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS lyrics"); err != nil {
			return fmt.Errorf("failed to reset cache table: %w", err)
		}
	}

	schema := fmt.Sprintf(`
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
		lyrics_file TEXT,
		created_at DATETIME DEFAULT (%s),
		updated_at DATETIME DEFAULT (%s)
	);
	CREATE INDEX IF NOT EXISTS idx_lyrics_cache_key ON lyrics(cache_key);
	CREATE INDEX IF NOT EXISTS idx_lyrics_updated_at ON lyrics(updated_at);
	PRAGMA user_version = %d;
	`, timestampFormat, timestampFormat, schemaVersion)

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize sqlite tables: %w", err)
	}
	return nil
}

// lyricColumns is the shared SELECT list for every read path.
const lyricColumns = `id, track_name, artist_name, album_name, duration,
	instrumental, plain_lyrics, synced_lyrics, lyrics_file`

// freshness builds the TTL predicate and its bind argument.
//
// An expiry of zero yields no predicate, so a disabled TTL costs no clause.
// The comparison runs entirely inside SQLite against the same textual format
// the columns are written with, which keeps the Go time.Time type out of the
// round trip.
//
// The modifier must be a fractional number of *seconds*: SQLite's date
// modifiers are days, hours, minutes, seconds, months and years only — a
// "-60 milliseconds" modifier is silently invalid and makes strftime return
// NULL, which would turn every comparison into a false and every row into a
// miss.
func (s *SQLiteStore) freshness() (clause string, modifier string) {
	if s.ttl <= 0 {
		return "", ""
	}
	return " AND updated_at >= strftime('%Y-%m-%d %H:%M:%f','now', ?)",
		fmt.Sprintf("-%.3f seconds", s.ttl.Seconds())
}

// scanLyric reads one row into an LRCLIB record. A NULL lyric column becomes
// a nil pointer so it marshals back out as JSON null.
func scanLyric(row interface{ Scan(...any) error }) (*model.LyricResponse, error) {
	var (
		id           int
		trackName    string
		artistName   string
		albumName    sql.NullString
		duration     sql.NullFloat64
		instrumental int
		plainLyrics  sql.NullString
		syncedLyrics sql.NullString
		lyricsFile   sql.NullString
	)

	if err := row.Scan(
		&id,
		&trackName,
		&artistName,
		&albumName,
		&duration,
		&instrumental,
		&plainLyrics,
		&syncedLyrics,
		&lyricsFile,
	); err != nil {
		return nil, err
	}

	return &model.LyricResponse{
		ID:           id,
		Name:         trackName,
		TrackName:    trackName,
		ArtistName:   artistName,
		AlbumName:    albumName.String,
		Duration:     duration.Float64,
		Instrumental: instrumental == 1,
		PlainLyrics:  model.StrPtr(plainLyrics.String),
		SyncedLyrics: model.StrPtr(syncedLyrics.String),
		LyricsFile:   model.StrPtr(lyricsFile.String),
	}, nil
}

// Get retrieves a cached lyric response by cacheKey, ignoring expired rows.
func (s *SQLiteStore) Get(cacheKey string) (*model.LyricResponse, error) {
	clause, modifier := s.freshness()

	query := `SELECT ` + lyricColumns + ` FROM lyrics WHERE cache_key = ?` + clause + ` LIMIT 1;`
	args := []any{cacheKey}
	if modifier != "" {
		args = append(args, modifier)
	}

	item, err := scanLyric(s.db.QueryRow(query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // cache miss (or expired)
		}
		return nil, err
	}
	return item, nil
}

// GetByID retrieves a cached lyric response by integer ID, ignoring expired
// rows.
func (s *SQLiteStore) GetByID(id int) (*model.LyricResponse, error) {
	clause, modifier := s.freshness()

	query := `SELECT ` + lyricColumns + ` FROM lyrics WHERE id = ?` + clause + ` LIMIT 1;`
	args := []any{id}
	if modifier != "" {
		args = append(args, modifier)
	}

	item, err := scanLyric(s.db.QueryRow(query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// Set stores or updates a lyric record and returns its row ID.
func (s *SQLiteStore) Set(cacheKey string, item *model.LyricResponse) (int, error) {
	instrumental := 0
	if item.Instrumental {
		instrumental = 1
	}

	query := fmt.Sprintf(`
	INSERT INTO lyrics (
		cache_key, track_name, artist_name, album_name, duration,
		instrumental, plain_lyrics, synced_lyrics, lyrics_file, updated_at
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, %s)
	ON CONFLICT(cache_key) DO UPDATE SET
		track_name=excluded.track_name,
		artist_name=excluded.artist_name,
		album_name=excluded.album_name,
		duration=excluded.duration,
		instrumental=excluded.instrumental,
		plain_lyrics=excluded.plain_lyrics,
		synced_lyrics=excluded.synced_lyrics,
		lyrics_file=excluded.lyrics_file,
		updated_at=%s
	RETURNING id;
	`, timestampFormat, timestampFormat)

	// The *string fields go to the driver as-is: database/sql dereferences
	// pointers and maps nil to SQL NULL, preserving the null/empty distinction.
	var id int
	if err := s.db.QueryRow(query,
		cacheKey,
		item.TrackName,
		item.ArtistName,
		item.AlbumName,
		item.Duration,
		instrumental,
		item.PlainLyrics,
		item.SyncedLyrics,
		item.LyricsFile,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("failed to insert/update lyric cache: %w", err)
	}

	item.ID = id
	return id, nil
}

// PruneExpired deletes rows past the TTL and reports how many were removed.
func (s *SQLiteStore) PruneExpired() (int64, error) {
	_, modifier := s.freshness()
	if modifier == "" {
		return 0, nil
	}

	result, err := s.db.Exec(
		`DELETE FROM lyrics WHERE updated_at < strftime('%Y-%m-%d %H:%M:%f','now', ?)`,
		modifier,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to prune expired cache rows: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to count pruned rows: %w", err)
	}
	return removed, nil
}

// TTL reports the configured expiry (zero means never).
func (s *SQLiteStore) TTL() time.Duration { return s.ttl }

// Close closes the underlying SQLite database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
