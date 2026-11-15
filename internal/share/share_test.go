package share

import (
	"path/filepath"
	"testing"
)

func TestShareCreateGetConsumeDelete(t *testing.T) {
	dir := t.TempDir()
	store := &Store{DataDir: dir}
	path := filepath.Join(dir, "file.bin")

	link, err := store.Create(path, "file.bin", 3600, 2)
	if err != nil {
		t.Fatal(err)
	}
	if link.Token == "" || link.Name != "file.bin" {
		t.Fatalf("unexpected link: %+v", link)
	}

	got, ok, err := store.Get(link.Token)
	if err != nil || !ok || got.Token != link.Token {
		t.Fatalf("get: ok=%v err=%v got=%+v", ok, err, got)
	}

	if _, ok, err := store.Consume(link.Token); err != nil || !ok {
		t.Fatalf("consume1: ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.Consume(link.Token); err != nil || !ok {
		t.Fatalf("consume2: ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.Consume(link.Token); err != nil || ok {
		t.Fatalf("consume3 should fail: ok=%v err=%v", ok, err)
	}

	link2, err := store.Create(path, "file.bin", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(link2.Token); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Get(link2.Token); err != nil || ok {
		t.Fatalf("deleted link still present")
	}
}
