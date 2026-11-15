package manifest

import "testing"

func TestParseYAML_valid(t *testing.T) {
	raw := []byte(`
schema: 1
id: org.fleet.rssfeeds
name: RSS Manager
version: 1.0.0
api: ">=1.0.0 <2.0.0"
engines: [rtorrent]
capabilities: [torrents.read, torrents.add, jobs.schedule, storage.write, http.fetch, tools.mktorrent]
frontend:
  type: declarative
  entry: ui/extension.json
backend:
  type: process
  executable: bin/rss-worker
  protocol: jsonrpc-2
`)
	m, err := ParseYAML(raw)
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	if m.ID != "org.fleet.rssfeeds" {
		t.Fatalf("id = %q", m.ID)
	}
	if !m.HasCapability("http.fetch") {
		t.Fatal("expected http.fetch")
	}
	if !m.HasCapability("tools.mktorrent") {
		t.Fatal("expected tools.mktorrent")
	}
}

func TestParseYAML_rejectsUnknownCapability(t *testing.T) {
	raw := []byte(`
schema: 1
id: org.fleet.bad
name: Bad
version: 0.0.1
api: ">=1.0.0"
capabilities: [root.exec]
backend:
  type: process
  executable: bin/x
  protocol: jsonrpc-2
`)
	if _, err := ParseYAML(raw); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseYAML_rejectsAbsoluteExecutable(t *testing.T) {
	raw := []byte(`
schema: 1
id: org.fleet.bad
name: Bad
version: 0.0.1
api: ">=1.0.0"
backend:
  type: process
  executable: /usr/bin/evil
  protocol: jsonrpc-2
`)
	if _, err := ParseYAML(raw); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseYAML_requiresFrontendOrBackend(t *testing.T) {
	raw := []byte(`
schema: 1
id: org.fleet.empty
name: Empty
version: 0.0.1
api: ">=1.0.0"
`)
	if _, err := ParseYAML(raw); err == nil {
		t.Fatal("expected error")
	}
}
