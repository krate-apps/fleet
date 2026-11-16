package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOverlayNonEmpty(t *testing.T) {
	base := Paths{Mktorrent: "/yaml/mk", Ffmpeg: "/yaml/ff"}
	over := Paths{Ffmpeg: "/ui/ff", Unrar: "/ui/unrar"}
	got := OverlayNonEmpty(base, over)
	if got.Mktorrent != "/yaml/mk" || got.Ffmpeg != "/ui/ff" || got.Unrar != "/ui/unrar" {
		t.Fatalf("%+v", got)
	}
}

func TestValidatePath(t *testing.T) {
	if err := ValidatePath(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePath("relative"); err == nil {
		t.Fatal("expected absolute path error")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePath(bin); err != nil {
		t.Fatal(err)
	}
}

func TestSetPathsThreadSafe(t *testing.T) {
	r := &Runner{MktorrentPath: "/old"}
	r.SetPaths(Paths{Mktorrent: "/new", Rclone: "/rclone"})
	p := r.Paths()
	if p.Mktorrent != "/new" || p.Rclone != "/rclone" {
		t.Fatalf("%+v", p)
	}
	st := r.Status()
	if _, ok := st["rclone"]; !ok {
		t.Fatal("missing rclone status")
	}
}
