package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BackupSQLite writes a consistent snapshot using VACUUM INTO (UPD-004).
// Prefer this over copying fleet.db while WAL mode may be active.
func (s *Store) BackupSQLite(ctx context.Context, destPath string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store closed")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		return err
	}
	_ = os.Remove(destPath)
	q := fmt.Sprintf(`VACUUM INTO '%s'`, escapeSQLString(destPath))
	if _, err := s.db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("sqlite backup: %w", err)
	}
	return nil
}

// BackupDir returns <dataDir>/backups.
func BackupDir(dataDir string) string {
	return filepath.Join(dataDir, "backups")
}

// NewBackupPath builds a timestamped backup filename under dataDir/backups.
func NewBackupPath(dataDir string) string {
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return filepath.Join(BackupDir(dataDir), "fleet-"+stamp+".db")
}

func escapeSQLString(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'')
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
