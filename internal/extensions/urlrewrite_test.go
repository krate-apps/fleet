package extensions

import (
	"path/filepath"
	"testing"
)

func TestURLRewriteStoreApply(t *testing.T) {
	dir := t.TempDir()
	store := &URLRewriteStore{DataDir: dir}
	if err := store.Set([]URLRewriteRule{
		{Name: "http-to-https", Pattern: `^http://`, Replacement: "https://", Enabled: true},
		{Name: "disabled", Pattern: `example`, Replacement: "nope", Enabled: false},
	}); err != nil {
		t.Fatal(err)
	}
	out, changed, matched, err := store.Apply("http://tracker.example/announce")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || out != "https://tracker.example/announce" || matched != "http-to-https" {
		t.Fatalf("got %q changed=%v matched=%q", out, changed, matched)
	}
	out2, changed2, matched2, err := store.Apply("https://already.secure/x")
	if err != nil {
		t.Fatal(err)
	}
	if changed2 || matched2 != "" || out2 != "https://already.secure/x" {
		t.Fatalf("unchanged got %q changed=%v matched=%q", out2, changed2, matched2)
	}
	if err := store.Set([]URLRewriteRule{{Name: "bad", Pattern: "(", Replacement: "", Enabled: true}}); err == nil {
		t.Fatal("expected invalid pattern error")
	}
	_ = filepath.Join(dir, "urlrewrite", "rules.json")
}
