package fsops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListMkdirRenameRemove(t *testing.T) {
	root := t.TempDir()
	m := &Manager{AllowedRoots: []string{root}, HideDot: true}
	if err := m.Mkdir(filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "f.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, cwd, err := m.List(filepath.Join(root, "a"))
	if err != nil || cwd == "" || len(entries) != 1 || entries[0].Name != "f.txt" {
		t.Fatalf("%v %#v %q", err, entries, cwd)
	}
	if err := m.Rename(filepath.Join(root, "a", "f.txt"), filepath.Join(root, "a", "g.txt")); err != nil {
		t.Fatal(err)
	}
	sum, n, err := m.ChecksumSHA256(filepath.Join(root, "a", "g.txt"))
	if err != nil || n != 2 || sum == "" {
		t.Fatalf("%v %d %s", err, n, sum)
	}
	if err := m.Remove(filepath.Join(root, "a", "g.txt"), false); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(root, true); err == nil {
		t.Fatal("expected refuse remove root")
	}
}

func TestStatFingerprintHardlink(t *testing.T) {
	root := t.TempDir()
	m := &Manager{AllowedRoots: []string{root}}
	src := filepath.Join(root, "a.bin")
	payload := make([]byte, 10000)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := m.Stat(src)
	if err != nil || st.Size != int64(len(payload)) || st.Device == 0 {
		t.Fatalf("%v %#v", err, st)
	}
	size, dig, err := m.Fingerprint(src, 4096)
	if err != nil || size != int64(len(payload)) || dig == "" {
		t.Fatalf("%v %d %s", err, size, dig)
	}
	size2, dig2, err := m.Fingerprint(src, 4096)
	if err != nil || dig2 != dig || size2 != size {
		t.Fatalf("fingerprint unstable: %v %s %s", err, dig, dig2)
	}
	dst := filepath.Join(root, "b.bin")
	if err := m.Hardlink(src, dst); err != nil {
		t.Fatal(err)
	}
	st2, err := m.Stat(dst)
	if err != nil || st2.Inode != st.Inode || st2.Device != st.Device {
		t.Fatalf("hardlink inode mismatch %#v %#v err=%v", st, st2, err)
	}
	if err := m.Hardlink(src, dst); err == nil {
		t.Fatal("expected existing dest refuse")
	}
}

func TestRefuseOutsideAndSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	m := &Manager{AllowedRoots: []string{root}}
	if _, _, err := m.Resolve(filepath.Join(outside, "x"), false); err == nil {
		t.Fatal("expected outside refuse")
	}
	target := filepath.Join(root, "real")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Resolve(link, true); err == nil {
		t.Fatal("expected symlink refuse")
	}
}
