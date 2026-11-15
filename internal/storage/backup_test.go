package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupSQLite(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB().Exec(`INSERT INTO meta(key, value) VALUES('backup_probe', '1')`); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "backups", "fleet-test.db")
	if err := store.BackupSQLite(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dest)
	if err != nil || st.Size() == 0 {
		t.Fatalf("backup missing: %v", err)
	}
	db, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'backup_probe'`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != "1" {
		t.Fatalf("value=%s", v)
	}
}
