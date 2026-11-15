package unpack

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRarGuardsWithoutBinary(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "missing.rar")
	if err := os.WriteFile(archive, []byte("not-a-rar"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out")
	_, err := Rar(archive, dest, []string{root}, "unrar-fleet-missing-binary")
	if err == nil {
		t.Fatal("expected missing binary error")
	}
}

func TestRarRefuseNonRarExt(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "a.zip")
	if err := os.WriteFile(archive, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Rar(archive, filepath.Join(root, "out"), []string{root}, "unrar"); err == nil {
		t.Fatal("expected extension refusal")
	}
}

func TestRarOutsideRoots(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	archive := filepath.Join(other, "a.rar")
	if err := os.WriteFile(archive, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Rar(archive, filepath.Join(root, "out"), []string{root}, "unrar"); err == nil {
		t.Fatal("expected roots refusal")
	}
}

func TestRarExtractIfUnrarAvailable(t *testing.T) {
	if _, err := exec.LookPath("unrar"); err != nil {
		t.Skip("unrar not on PATH")
	}
	// Without a real .rar fixture we only assert the binary path is resolvable
	// and path guards still apply to empty/invalid archives.
	root := t.TempDir()
	archive := filepath.Join(root, "bogus.rar")
	if err := os.WriteFile(archive, []byte("not-rar"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Rar(archive, filepath.Join(root, "out"), []string{root}, "")
	if err == nil {
		t.Fatal("expected unrar list failure on bogus archive")
	}
}
