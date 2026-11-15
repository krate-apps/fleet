package theme

import "testing"

func TestParseYAML_OK(t *testing.T) {
	m, err := ParseYAML([]byte(`
schema: 1
id: org.fleet.theme.slate
name: Slate
version: 1.0.0
colorScheme: dark
css: theme.css
`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ShortID() != "slate" {
		t.Fatalf("short id %q", m.ShortID())
	}
}

func TestParseYAML_BadScheme(t *testing.T) {
	_, err := ParseYAML([]byte(`
schema: 1
id: org.fleet.theme.x
name: X
version: 1.0.0
colorScheme: neon
css: theme.css
`))
	if err == nil {
		t.Fatal("expected error")
	}
}
