package themes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAndList(t *testing.T) {
	src := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "fleet.theme.yaml"), []byte(`
schema: 1
id: org.fleet.theme.slate
name: Slate
version: 1.0.0
colorScheme: dark
css: theme.css
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "theme.css"), []byte(`:root[data-fleet-theme="slate"]{--fleet-primary:#5b8def}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st := &Store{Root: root}
	inst, err := st.InstallLocal(src)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Manifest.ShortID() != "slate" {
		t.Fatalf("short %q", inst.Manifest.ShortID())
	}
	all, err := st.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("list %#v %v", all, err)
	}
	if err := st.Uninstall("slate"); err != nil {
		t.Fatal(err)
	}
	all, err = st.List()
	if err != nil || len(all) != 0 {
		t.Fatalf("after uninstall %#v %v", all, err)
	}
	if err := st.Uninstall("dark"); err == nil {
		t.Fatal("expected built-in uninstall refusal")
	}
}
