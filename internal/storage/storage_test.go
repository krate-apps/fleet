package storage

import (
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	var version string
	err = store.DB().QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if version != "6" {
		t.Fatalf("schema_version = %q", version)
	}

	for _, table := range []string{"jobs", "extensions", "peer_comments", "gateway_clients"} {
		var name string
		err = store.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}
}
