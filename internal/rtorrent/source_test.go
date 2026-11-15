package rtorrent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceCandidatesOrder(t *testing.T) {
	got := sourceCandidates("ABCDEF", "/sess", "/data/foo.torrent")
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
	if got[0] != filepath.Join("/sess", "ABCDEF.torrent") {
		t.Fatalf("session candidate first: %v", got)
	}
	if got[1] != "/data/foo.torrent" {
		t.Fatalf("tied second: %v", got)
	}
	// empty tied/session
	if c := sourceCandidates("ABCDEF", "", ""); len(c) != 0 {
		t.Fatalf("%v", c)
	}
}

func TestValidateSourceFile(t *testing.T) {
	root := t.TempDir()
	session := filepath.Join(root, "session")
	if err := os.MkdirAll(session, 0o700); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("A", 40)
	path := filepath.Join(session, hash+".torrent")
	if err := os.WriteFile(path, []byte("d8:announce"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSourceFile(path, []string{session}); err != nil {
		t.Fatalf("expected ok: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "evil.torrent")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSourceFile(outside, []string{session}); err == nil {
		t.Fatal("expected outside refuse")
	}

	link := filepath.Join(session, "link.torrent")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := validateSourceFile(link, []string{session}); err == nil {
		t.Fatal("expected symlink refuse")
	}
}

func TestSanitizeTorrentFilename(t *testing.T) {
	if got := sanitizeTorrentFilename(`Moana / "x"`); got != "Moana____x" {
		t.Fatalf("%q", got)
	}
	if got := sanitizeTorrentFilename(""); got != "torrent" {
		t.Fatalf("%q", got)
	}
}
