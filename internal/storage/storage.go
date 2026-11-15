package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps Fleet's SQLite database (sessions, preferences, extensions — never torrent authority).
type Store struct {
	db *sql.DB
}

// Open creates or opens the Fleet SQLite database under dataDir.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "fleet.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY NOT NULL,
	value TEXT NOT NULL
);
INSERT OR IGNORE INTO meta(key, value) VALUES ('schema_version', '1');

CREATE TABLE IF NOT EXISTS jobs (
	id           TEXT PRIMARY KEY NOT NULL,
	kind         TEXT NOT NULL,
	status       TEXT NOT NULL,
	extension_id TEXT NOT NULL DEFAULT '',
	progress     REAL NOT NULL DEFAULT 0,
	message      TEXT NOT NULL DEFAULT '',
	error        TEXT NOT NULL DEFAULT '',
	payload_json TEXT NOT NULL DEFAULT '{}',
	result_json  TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL,
	finished_at  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_extension ON jobs(extension_id);

CREATE TABLE IF NOT EXISTS extensions (
	id            TEXT PRIMARY KEY NOT NULL,
	name          TEXT NOT NULL,
	version       TEXT NOT NULL,
	path          TEXT NOT NULL,
	state         TEXT NOT NULL,
	manifest_json TEXT NOT NULL,
	error         TEXT NOT NULL DEFAULT '',
	installed_at  TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS peer_comments (
	address    TEXT PRIMARY KEY NOT NULL,
	comment    TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS gateway_clients (
	id           TEXT PRIMARY KEY NOT NULL,
	kind         TEXT NOT NULL,
	name         TEXT NOT NULL,
	url          TEXT NOT NULL,
	username     TEXT NOT NULL DEFAULT '',
	password     TEXT NOT NULL DEFAULT '',
	enabled      INTEGER NOT NULL DEFAULT 1,
	is_default   INTEGER NOT NULL DEFAULT 0,
	journal_unit TEXT NOT NULL DEFAULT '',
	log_path     TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_gateway_clients_kind ON gateway_clients(kind);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	// Existing installs may predate later gateway columns.
	for _, col := range []struct {
		sql string
		err string
	}{
		{`ALTER TABLE gateway_clients ADD COLUMN is_default INTEGER NOT NULL DEFAULT 0`, "is_default"},
		{`ALTER TABLE gateway_clients ADD COLUMN journal_unit TEXT NOT NULL DEFAULT ''`, "journal_unit"},
		{`ALTER TABLE gateway_clients ADD COLUMN log_path TEXT NOT NULL DEFAULT ''`, "log_path"},
	} {
		if _, err := s.db.Exec(col.sql); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return fmt.Errorf("migrate gateway %s: %w", col.err, err)
			}
		}
	}
	if _, err := s.db.Exec(`UPDATE meta SET value = '6' WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("bump schema_version: %w", err)
	}
	return nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the underlying handle for packages that need raw SQL during early milestones.
func (s *Store) DB() *sql.DB {
	return s.db
}

// GetMeta reads a value from the meta key/value table.
func (s *Store) GetMeta(key string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("store unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("key required")
	}
	var value string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// PutMeta upserts a meta key/value pair.
func (s *Store) PutMeta(key, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key required")
	}
	_, err := s.db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
