package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpTorrentRequiresConfig(t *testing.T) {
	r := &Runner{}
	_, err := r.DumpTorrent(context.Background(), []byte("x"))
	if err == nil {
		t.Fatal("expected missing config error")
	}
}

func TestDumpTorrentParsesJSONBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "dumptorrent")
	script := "#!/bin/sh\necho '{\"name\":\"demo\",\"size\":123}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{DumptorrentPath: bin}
	res, err := r.DumpTorrent(context.Background(), []byte("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Parsed["name"] != "demo" {
		t.Fatalf("parsed=%v", res.Parsed)
	}
}

func TestStatusReportsMissing(t *testing.T) {
	r := &Runner{}
	st := r.Status()
	m := st["dumptorrent"].(map[string]any)
	if m["available"] != false {
		t.Fatalf("%v", m)
	}
}

func TestCreateTorrentRequiresRoots(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "mktorrent")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{MktorrentPath: bin, ArtifactsDir: t.TempDir()}
	_, err := r.CreateTorrent(context.Background(), CreateInput{
		Paths:    []string{src},
		Announce: []string{"https://tracker.example/announce"},
	})
	if err == nil || !strings.Contains(err.Error(), "download_roots") {
		t.Fatalf("expected roots error, got %v", err)
	}
}

func TestCreateTorrentJSONSuccess(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "payload.bin")
	if err := os.WriteFile(src, []byte("hello-fleet"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifacts := t.TempDir()
	bin := filepath.Join(t.TempDir(), "mktorrent")
	// Fake JSON mktorrent: honor -o and print JSON.
	script := `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then out="$2"; shift 2; continue; fi
  shift
done
printf 'd4:infod4:name4:demoee' > "$out"
echo "{\"ok\":true,\"path\":\"$out\"}"
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{
		MktorrentPath: bin,
		AllowedRoots:  []string{root},
		ArtifactsDir:  artifacts,
	}
	res, err := r.CreateTorrent(context.Background(), CreateInput{
		Paths:    []string{src},
		Announce: []string{"https://tracker.example/announce"},
		Private:  true,
		Name:     "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TorrentBase64 == "" || !res.JSONCLI {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, err := os.Stat(res.OutputPath); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTorrentRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "mktorrent")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.bin")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &Runner{
		MktorrentPath: bin,
		AllowedRoots:  []string{root},
		ArtifactsDir:  t.TempDir(),
	}
	_, err := r.CreateTorrent(context.Background(), CreateInput{
		Paths:    []string{outside},
		Announce: []string{"https://tracker.example/announce"},
	})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected outside error, got %v", err)
	}
}
