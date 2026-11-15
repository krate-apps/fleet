package rtorrent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDataFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "payload.bin")
	if err := os.WriteFile(f, []byte("hello-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateDataFile(f, []string{root}); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "x.bin")
	_ = os.WriteFile(outside, []byte("x"), 0o600)
	if err := validateDataFile(outside, []string{root}); err == nil {
		t.Fatal("expected outside roots refusal")
	}
	if err := validateDataFile(f, nil); err == nil {
		t.Fatal("expected empty roots refusal")
	}
}
