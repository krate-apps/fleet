package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestHostRefreshActivate(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "rss")
	if err := os.MkdirAll(filepath.Join(pkg, "ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`
schema: 1
id: org.fleet.rssfeeds
name: RSS Manager
version: 0.1.0
api: ">=1.0.0 <2.0.0"
engines: [rtorrent]
capabilities: [torrents.read, http.fetch, jobs.schedule, storage.write]
frontend:
  type: declarative
  entry: ui/extension.json
`)
	// Frontend-only fixture: worker supervision is covered in worker_test.go.
	if err := os.WriteFile(filepath.Join(pkg, "fleet.ext.yaml"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	uiJSON := []byte(`{
  "schema": 1,
  "extensionId": "org.fleet.rssfeeds",
  "pages": [{"id": "feeds", "title": "Feeds", "route": "/ext/org.fleet.rssfeeds/feeds"}]
}`)
	if err := os.WriteFile(filepath.Join(pkg, "ui", "extension.json"), uiJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	h := NewHost(store, root)
	if err := h.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	list := h.List()
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
	if list[0].ID != "org.fleet.rssfeeds" || list[0].UI == nil {
		t.Fatalf("unexpected record: %+v", list[0])
	}

	rec, err := h.Activate("org.fleet.rssfeeds")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if rec.State != StateActive {
		t.Fatalf("state = %q", rec.State)
	}

	// Second refresh preserves active state.
	if err := h.Refresh(); err != nil {
		t.Fatalf("Refresh2: %v", err)
	}
	got, err := h.Get("org.fleet.rssfeeds")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateActive {
		t.Fatalf("state after refresh = %q", got.State)
	}

	if _, err := h.Deactivate("org.fleet.rssfeeds"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
}

func TestHostRefresh_invalidPackage(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "broken")
	if err := os.MkdirAll(pkg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "fleet.ext.yaml"), []byte("schema: 1\nid: bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := NewHost(nil, root)
	if err := h.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	list := h.List()
	if len(list) != 1 || list[0].State != StateError {
		t.Fatalf("expected error record, got %+v", list)
	}
}

func TestHostRefresh_skipsEmptyDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "org.fleet.ghost"), 0o700); err != nil {
		t.Fatal(err)
	}
	h := NewHost(nil, root)
	if err := h.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if list := h.List(); len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func TestDataDir_usesExtensionData(t *testing.T) {
	got := DataDir("/var/lib/fleet", "org.fleet/rss")
	want := filepath.Join("/var/lib/fleet", "extension-data", "org.fleet_rss")
	if got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}
}
