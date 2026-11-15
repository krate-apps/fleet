package metainfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndParseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(src, []byte("hello fleet"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "hello.torrent")
	sum, err := CreateTorrent(CreateOptions{
		Paths:       []string{src},
		Announce:    []string{"http://127.0.0.1:9/announce"},
		PieceLength: 16384,
		OutputPath:  out,
		Comment:     "lab",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sum.InfoHash == "" || sum.Name != "hello.txt" {
		t.Fatalf("bad summary: %+v", sum)
	}
	got, err := ParseTorrentFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got.InfoHash != sum.InfoHash {
		t.Fatalf("hash mismatch %s vs %s", got.InfoHash, sum.InfoHash)
	}
	if got.Length != int64(len("hello fleet")) {
		t.Fatalf("length %d", got.Length)
	}
}

func TestBencodeDictRoundTrip(t *testing.T) {
	in := map[string]any{"b": int64(2), "a": "x"}
	raw, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["a"] != "x" || m["b"].(int64) != 2 {
		t.Fatalf("%v", m)
	}
}
