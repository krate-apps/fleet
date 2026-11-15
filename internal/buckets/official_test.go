package buckets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestEnsureOfficialBucket(t *testing.T) {
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

	bucketRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(bucketRoot, "bucket.json"), []byte(`{
  "schema": 1,
  "id": "org.fleet.official",
  "name": "Fleet Official",
  "official": true
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bucketRoot, "packages", "demo", "ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bucketRoot, "packages", "demo", "fleet.ext.yaml"), []byte(`
schema: 1
id: org.fleet.demo
name: Demo
version: 0.1.0
api: ">=1.0.0"
frontend:
  type: declarative
  entry: ui/extension.json
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bucketRoot, "packages", "demo", "ui", "extension.json"), []byte(`{
  "schema": 1,
  "extensionId": "org.fleet.demo",
  "pages": [{"id": "home", "title": "Home", "route": "/ext/org.fleet.demo/home"}]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bucketRoot, "bucket"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bucketRoot, "bucket", "org.fleet.demo.json"), []byte(`{
  "schema": 1,
  "kind": "extension",
  "id": "org.fleet.demo",
  "name": "Demo",
  "version": "0.1.0",
  "api": ">=1.0.0",
  "source": { "path": "packages/demo" }
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	rec, err := reg.EnsureOfficialBucket(EnsureOfficialOptions{Path: bucketRoot})
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil || !rec.Official || !rec.Enabled {
		t.Fatalf("%+v", rec)
	}
	// Second call must not re-enable after user disables.
	if _, err := reg.SetEnabled(OfficialBucketID, false); err != nil {
		t.Fatal(err)
	}
	rec2, err := reg.EnsureOfficialBucket(EnsureOfficialOptions{Path: bucketRoot})
	if err != nil {
		t.Fatal(err)
	}
	if rec2.Enabled {
		t.Fatal("expected disabled to stick")
	}
	// Disabled config skips.
	if rec3, err := reg.EnsureOfficialBucket(EnsureOfficialOptions{Path: bucketRoot, Disabled: true}); err != nil || rec3 != nil {
		t.Fatalf("disabled=%v err=%v", rec3, err)
	}
	cat, err := reg.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 0 {
		t.Fatalf("disabled bucket should hide catalog: %+v", cat)
	}
	if _, err := reg.SetEnabled(OfficialBucketID, true); err != nil {
		t.Fatal(err)
	}
	cat, err = reg.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 1 || !cat[0].Installable {
		t.Fatalf("%+v", cat)
	}
}
