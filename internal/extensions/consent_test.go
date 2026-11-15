package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestIsOfficialExtensionExactAllowlist(t *testing.T) {
	if !IsOfficialExtension("org.fleet.eventhooks") {
		t.Fatal("known official id should pass")
	}
	if IsOfficialExtension("org.fleet.evil") {
		t.Fatal("spoofed org.fleet.* must not skip consent")
	}
	if IsOfficialExtension("com.example.ext") {
		t.Fatal("third-party must require consent")
	}
}

func TestThirdPartyConsentGate(t *testing.T) {
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	root := t.TempDir()
	pkg := filepath.Join(root, "third")
	if err := os.MkdirAll(filepath.Join(pkg, "ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "fleet.ext.yaml"), []byte(`
schema: 1
id: org.example.third
name: Third
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
  "extensionId": "org.example.third",
  "pages": [{"id": "home", "title": "Home", "route": "/ext/org.example.third/home"}]
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	h := NewHost(store, root)
	if err := h.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Activate("org.example.third"); err == nil {
		t.Fatal("expected consent required")
	}
	if err := h.GrantConsent("org.example.third"); err != nil {
		t.Fatal(err)
	}
	rec, err := h.Activate("org.example.third")
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != StateActive || !rec.Consented {
		t.Fatalf("%+v", rec)
	}
	if !IsOfficialExtension("org.fleet.rssfeeds") || IsOfficialExtension("org.example.x") {
		t.Fatal("official allowlist check")
	}
}
