package unpack

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipExtractAndGuards(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "a.zip")
	if err := writeTestZip(archive, map[string]string{"ok.txt": "hi", "dir/x.txt": "x"}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out")
	n, err := Zip(archive, dest, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("extracted=%d", n)
	}
	b, err := os.ReadFile(filepath.Join(dest, "ok.txt"))
	if err != nil || string(b) != "hi" {
		t.Fatalf("ok.txt: %v %q", err, b)
	}

	evil := filepath.Join(root, "evil.zip")
	if err := writeTestZip(evil, map[string]string{"../escape.txt": "no"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Zip(evil, dest, []string{root}); err == nil {
		t.Fatal("expected traversal refusal")
	}

	outside := t.TempDir()
	if _, err := Zip(archive, filepath.Join(outside, "x"), []string{root}); err == nil {
		t.Fatal("expected dest outside roots refusal")
	}
}

func writeTestZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return err
		}
	}
	return zw.Close()
}
