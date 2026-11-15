package sysinfo

import (
	"os"
	"testing"
)

func TestDiskForPathTemp(t *testing.T) {
	dir := t.TempDir()
	u, err := DiskForPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if u.Total == 0 || u.Path == "" {
		t.Fatalf("unexpected usage: %+v", u)
	}
}

func TestDiskForRootsSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	out := DiskForRoots([]string{dir, "/no/such/fleet/root/definitely-missing", "", "."})
	if len(out) != 1 {
		t.Fatalf("got %d entries: %+v", len(out), out)
	}
}

func TestLoad(t *testing.T) {
	if _, err := os.Stat("/proc/loadavg"); err != nil {
		t.Skip("no /proc/loadavg")
	}
	l, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if l.Load1 < 0 || l.Load5 < 0 || l.Load15 < 0 {
		t.Fatalf("negative load: %+v", l)
	}
}
