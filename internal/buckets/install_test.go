package buckets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestInstallArchiveAndURL(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reg, err := NewRegistry(store, data)
	if err != nil {
		t.Fatal(err)
	}

	manifest := "schema: 1\nid: org.fleet.fromarchive\nname: FromArchive\nversion: 0.1.0\napi: \">=1.0.0\"\nfrontend:\n  type: declarative\n  entry: ui/extension.json\n"
	ui := `{"schema":1,"extensionId":"org.fleet.fromarchive","pages":[{"id":"h","title":"H","route":"/ext/org.fleet.fromarchive/h"}]}`
	archivePath, sum := writeTestArchive(t, data, map[string]string{
		"pkg/fleet.ext.yaml":    manifest,
		"pkg/ui/extension.json": ui,
	})

	m, err := reg.InstallArchive(archivePath, sum)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "org.fleet.fromarchive" {
		t.Fatalf("%+v", m)
	}
	if _, err := os.Stat(filepath.Join(reg.InstallRoot(), "org.fleet.fromarchive", "fleet.ext.yaml")); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	// Non-GitHub https URL is allowed with sha256.
	m2, err := reg.InstallURL(context.Background(), srv.URL+"/pkg.tar.gz", sum, DownloadOptions{HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if m2.ID != "org.fleet.fromarchive" {
		t.Fatalf("%+v", m2)
	}

	// GitHub-looking host that is not a release asset must fail before download.
	if _, err := reg.InstallURL(context.Background(), "https://github.com/org/repo/archive/main.tar.gz", sum, DownloadOptions{}); err == nil {
		t.Fatal("expected github diagnostic")
	}

	if err := reg.UninstallPackage("org.fleet.fromarchive"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(reg.InstallRoot(), "org.fleet.fromarchive")); !os.IsNotExist(err) {
		t.Fatalf("expected uninstall to remove package: %v", err)
	}
}

func TestInstallURLRequiresSHA(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reg, err := NewRegistry(store, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.InstallURL(context.Background(), "https://example.com/x.tar.gz", "", DownloadOptions{}); err == nil {
		t.Fatal("expected sha required")
	}
}

func TestInstallFromBucket_fallsBackToArchives(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reg, err := NewRegistry(store, data)
	if err != nil {
		t.Fatal(err)
	}

	bucket := filepath.Join(data, "bucket")
	for _, d := range []string{
		filepath.Join(bucket, "bucket"),
		filepath.Join(bucket, "archives"),
	} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bucket, "bucket.json"), []byte(`{
  "schema": 1,
  "id": "org.fleet.archivebucket",
  "name": "Archive Bucket"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\nid: org.fleet.demo\nname: Demo\nversion: 0.1.0\napi: \">=1.0.0\"\nfrontend:\n  type: declarative\n  entry: ui/extension.json\n"
	ui := `{"schema":1,"extensionId":"org.fleet.demo","pages":[{"id":"h","title":"H","route":"/ext/org.fleet.demo/h"}]}`
	archivePath, sum := writeTestArchive(t, data, map[string]string{
		"pkg/fleet.ext.yaml":    manifest,
		"pkg/ui/extension.json": ui,
	})
	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bucket, "archives", "org.fleet.demo-linux-amd64.tar.gz"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	listing := `{
  "schema": 1,
  "kind": "extension",
  "id": "org.fleet.demo",
  "name": "Demo",
  "version": "0.1.0",
  "api": ">=1.0.0",
  "source": { "path": "packages/missing" },
  "architecture": {
    "linux-amd64": {
      "url": "https://github.com/krate-apps/fleet/releases/download/v0.1.11/org.fleet.demo-linux-amd64.tar.gz",
      "hash": "sha256:` + sum + `"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(bucket, "bucket", "org.fleet.demo.json"), []byte(listing), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, err := reg.AddLocal(bucket)
	if err != nil {
		t.Fatal(err)
	}
	m, err := reg.InstallFromBucket(rec.ID, "org.fleet.demo")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "org.fleet.demo" {
		t.Fatalf("%+v", m)
	}
}
