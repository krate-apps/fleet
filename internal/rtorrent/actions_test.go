package rtorrent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssertUnderRoots(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "shows", "a.mkv")
	if err := os.MkdirAll(filepath.Dir(inside), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := assertUnderRoots(inside, []string{root}); err != nil {
		t.Fatalf("expected inside ok: %v", err)
	}
	if err := assertUnderRoots(root, []string{root}); err == nil {
		t.Fatal("expected exact download root to be refused")
	}
	outside := filepath.Join(t.TempDir(), "evil")
	if err := assertUnderRoots(outside, []string{root}); err == nil {
		t.Fatal("expected outside path to fail")
	}
	if err := assertUnderRoots(inside, nil); err == nil {
		t.Fatal("expected empty roots to fail via caller; assert itself fails containment")
	}
}

func TestAssertUnderRootsRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	evil := filepath.Join(root, "..", "outside")
	if err := assertUnderRoots(evil, []string{root}); err == nil {
		t.Fatal("expected .. escape to fail")
	}
}
