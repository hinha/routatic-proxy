package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/routatic/proxy/internal/cacheusage"
	"github.com/routatic/proxy/internal/history"
)

func TestRequestsRoundTripCacheUsage(t *testing.T) {
	db := openTestDatabase(t)
	requests := NewRequests(db)

	want := history.RequestRecord{
		ID:           "cache-request",
		Model:        "muse-spark-1.3-contributor",
		Provider:     "opencode-go",
		StartTime:    time.Now().UTC().Truncate(time.Millisecond),
		InputTokens:  20,
		OutputTokens: 5,
		CacheUsage:   cacheusage.Usage{ReadTokens: 75, CreationTokens: 25, Reported: true},
		Success:      true,
	}
	if err := requests.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := requests.Last(1)
	if err != nil {
		t.Fatalf("Last() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Last() returned %d records, want 1", len(got))
	}
	if got[0].CacheUsage != want.CacheUsage {
		t.Fatalf("CacheUsage = %+v, want %+v", got[0].CacheUsage, want.CacheUsage)
	}
}

func TestRequestsRoundTripUnreportedCacheUsage(t *testing.T) {
	db := openTestDatabase(t)
	requests := NewRequests(db)

	if err := requests.Insert(history.RequestRecord{ID: "no-cache-data", Model: "model", StartTime: time.Now().UTC()}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := requests.Last(1)
	if err != nil {
		t.Fatalf("Last() error = %v", err)
	}
	if got[0].CacheUsage.Reported {
		t.Fatalf("CacheUsage = %+v, want unreported usage", got[0].CacheUsage)
	}
}

func TestOpenMigratesCacheUsageColumnsOnExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	_, err = legacy.Exec(`
		CREATE TABLE requests (
			id TEXT PRIMARY KEY,
			model TEXT NOT NULL,
			provider TEXT,
			scenario TEXT,
			start_time TIMESTAMP NOT NULL,
			duration_ms INTEGER,
			input_tokens INTEGER,
			output_tokens INTEGER,
			streaming INTEGER,
			success INTEGER,
			error_msg TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		_ = legacy.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := Open(Config{DatabasePath: path, WALEnabled: true})
	if err != nil {
		t.Fatalf("Open() legacy database error = %v", err)
	}
	defer func() { _ = db.Close() }()

	var read, created, reported int
	if err := db.DB().QueryRow(`
		SELECT cache_read_input_tokens, cache_creation_input_tokens, cache_usage_reported
		FROM requests LIMIT 1
	`).Scan(&read, &created, &reported); err != sql.ErrNoRows {
		t.Fatalf("cache columns query error = %v, want sql.ErrNoRows", err)
	}

	if err := NewRequests(db).Insert(history.RequestRecord{
		ID:         "migrated-request",
		Model:      "model",
		StartTime:  time.Now().UTC(),
		CacheUsage: cacheusage.Usage{ReadTokens: 8, CreationTokens: 2, Reported: true},
	}); err != nil {
		t.Fatalf("Insert() after migration error = %v", err)
	}
}

func openTestDatabase(t *testing.T) *Database {
	t.Helper()

	db, err := Open(Config{
		DatabasePath: filepath.Join(t.TempDir(), "test.db"),
		WALEnabled:   true,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
