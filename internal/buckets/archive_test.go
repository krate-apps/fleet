package buckets

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeTestArchive(t *testing.T, dir string, files map[string]string) (path, sum string) {
	t.Helper()
	path = filepath.Join(dir, "pkg.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	return path, hex.EncodeToString(h[:])
}

func TestExtractTarGzAndVerify(t *testing.T) {
	dir := t.TempDir()
	manifest := "schema: 1\nid: org.fleet.demo\nname: Demo\nversion: 0.1.0\napi: \">=1.0.0\"\nfrontend:\n  type: declarative\n  entry: ui/extension.json\n"
	ui := `{"schema":1,"extensionId":"org.fleet.demo","pages":[{"id":"h","title":"H","route":"/ext/org.fleet.demo/h"}]}`
	path, sum := writeTestArchive(t, dir, map[string]string{
		"pkg/fleet.ext.yaml":  manifest,
		"pkg/ui/extension.json": ui,
	})
	if err := VerifySHA256(path, sum); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected sha mismatch")
	}
	out := filepath.Join(dir, "out")
	if err := ExtractTarGz(path, out); err != nil {
		t.Fatal(err)
	}
	root, err := FindPackageRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "fleet.ext.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "../evil", Mode: 0o644, Size: 1}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
	if err := ExtractTarGz(path, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
