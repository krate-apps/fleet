package ui

import "testing"

func TestParseJSON_valid(t *testing.T) {
	raw := []byte(`{
  "schema": 1,
  "extensionId": "org.fleet.rssfeeds",
  "pages": [{"id": "feeds", "title": "Feeds", "route": "/ext/org.fleet.rssfeeds/feeds"}],
  "actions": [{"id": "refresh", "label": "Refresh", "target": "global"}]
}`)
	c, err := ParseJSON(raw)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if len(c.Pages) != 1 {
		t.Fatalf("pages = %d", len(c.Pages))
	}
}

func TestParseJSON_rejectsBadRoute(t *testing.T) {
	raw := []byte(`{
  "schema": 1,
  "extensionId": "org.fleet.rssfeeds",
  "pages": [{"id": "x", "title": "X", "route": "/admin"}]
}`)
	if _, err := ParseJSON(raw); err == nil {
		t.Fatal("expected error")
	}
}
