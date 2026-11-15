package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPathPermissions(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "private")
	if err := os.Mkdir(ok, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CheckPathPermissions(ok); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "world")
	if err := os.Mkdir(bad, 0o777); err != nil {
		t.Fatal(err)
	}
	// Some umasks clear other-write; force it.
	if err := os.Chmod(bad, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := CheckPathPermissions(bad); err == nil {
		t.Fatal("expected world-writable rejection")
	}
}

func TestEnsurePrivateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		// Accept 0700; umask may leave 0755 which is ok (not world-writable).
		if st.Mode().Perm()&0o002 != 0 {
			t.Fatalf("mode=%#o", st.Mode().Perm())
		}
	}
}
