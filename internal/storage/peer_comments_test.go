package storage

import (
	"path/filepath"
	"testing"
)

func TestPeerComments(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.SetPeerComment("1.2.3.4", "seedbox"); err != nil {
		t.Fatal(err)
	}
	m, err := store.ListPeerComments([]string{"1.2.3.4", "9.9.9.9", "1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	if m["1.2.3.4"] != "seedbox" {
		t.Fatalf("got %#v", m)
	}
	if _, ok := m["9.9.9.9"]; ok {
		t.Fatal("unexpected comment")
	}
	if err := store.SetPeerComment("1.2.3.4", ""); err != nil {
		t.Fatal(err)
	}
	m, err = store.ListPeerComments([]string{"1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("expected delete, got %#v", m)
	}
}
