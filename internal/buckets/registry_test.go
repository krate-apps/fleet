package buckets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestRegistryAddCatalogInstall(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	bucketRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(bucketRoot, "bucket.json"), []byte(`{
  "schema": 1,
  "id": "org.fleet.testbucket",
  "name": "Test Bucket"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(bucketRoot, "packages", "demo")
	if err := os.MkdirAll(filepath.Join(pkg, "ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "fleet.ext.yaml"), []byte(`
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
	if err := os.WriteFile(filepath.Join(pkg, "ui", "extension.json"), []byte(`{
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

	reg, err := NewRegistry(store, data)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := reg.AddLocal(bucketRoot)
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "org.fleet.testbucket" {
		t.Fatalf("%+v", rec)
	}
	cat, err := reg.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 1 || cat[0].Origin != "org.fleet.testbucket/org.fleet.demo" {
		t.Fatalf("%+v", cat)
	}
	m, err := reg.InstallFromBucket("org.fleet.testbucket", "org.fleet.demo")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "org.fleet.demo" {
		t.Fatalf("%+v", m)
	}
	installed := filepath.Join(reg.InstallRoot(), "org.fleet.demo", "fleet.ext.yaml")
	if _, err := os.Stat(installed); err != nil {
		t.Fatal(err)
	}
}

func TestThemesCatalogAndResolve(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	bucketRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(bucketRoot, "bucket.json"), []byte(`{
  "schema": 1,
  "id": "org.fleet.testthemes",
  "name": "Test Themes"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(bucketRoot, "packages", "themes", "slate")
	if err := os.MkdirAll(pkg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "fleet.theme.yaml"), []byte(`
schema: 1
id: org.fleet.theme.slate
name: Slate
version: 1.0.0
colorScheme: dark
css: theme.css
uiThemeId: slate
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "theme.css"), []byte(`:root[data-fleet-theme="slate"]{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bucketRoot, "bucket"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bucketRoot, "bucket", "org.fleet.theme.slate.json"), []byte(`{
  "schema": 1,
  "kind": "theme",
  "id": "org.fleet.theme.slate",
  "name": "Slate",
  "version": "1.0.0",
  "source": { "path": "packages/themes/slate" }
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	reg, err := NewRegistry(store, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.AddLocal(bucketRoot); err != nil {
		t.Fatal(err)
	}
	cat, err := reg.ThemesCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 1 || !cat[0].Installable || cat[0].ID != "org.fleet.theme.slate" {
		t.Fatalf("%+v", cat)
	}
	dir, err := reg.ResolveThemePackageDir("org.fleet.testthemes", "org.fleet.theme.slate")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fleet.theme.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ResolveThemePackageDir("org.fleet.testthemes", "missing"); err == nil {
		t.Fatal("expected missing theme error")
	}
}
