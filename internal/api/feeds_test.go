package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/sync"
)

func TestFeedsIndexAndXML(t *testing.T) {
	eng := sync.NewEngine(nil, nil, sync.Options{})
	srv := New(config.Default(), nil, nil, eng)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feeds", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index status=%d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	modes, ok := body["modes"].([]any)
	if !ok || len(modes) != 6 {
		t.Fatalf("modes=%#v", body["modes"])
	}
	first, _ := modes[0].(map[string]any)
	if path, _ := first["path"].(string); path != "/api/v1/feeds/all.xml" {
		t.Fatalf("path without prefix = %q", path)
	}

	rec = httptest.NewRecorder()
	reqPrefixed := httptest.NewRequest(http.MethodGet, "/api/v1/feeds", nil)
	reqPrefixed.Header.Set("X-Forwarded-Prefix", "/trevor/fleet")
	srv.Handler().ServeHTTP(rec, reqPrefixed)
	if rec.Code != http.StatusOK {
		t.Fatalf("prefixed index status=%d", rec.Code)
	}
	body = map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	modes, _ = body["modes"].([]any)
	first, _ = modes[0].(map[string]any)
	if path, _ := first["path"].(string); path != "/api/v1/feeds/all.xml" {
		t.Fatalf("path must stay app-relative = %q", path)
	}
	if url, _ := first["url"].(string); !strings.Contains(url, "/trevor/fleet/api/v1/feeds/all.xml") {
		t.Fatalf("url with prefix = %q", url)
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feeds/all.xml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("xml status=%d body=%s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/rss+xml") {
		t.Fatalf("content-type=%q", ct)
	}
	if !strings.Contains(rec.Body.String(), `<rss version="2.0">`) {
		t.Fatalf("body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feeds/nope.xml", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad mode status=%d", rec.Code)
	}
}

func TestFeedsSettingsOverrideURL(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	eng := sync.NewEngine(nil, nil, sync.Options{})
	srv := New(config.Default(), nil, nil, eng, Options{Store: store})

	rec := httptest.NewRecorder()
	body := `{"publicHost":"feeds.example","httpPort":8080,"httpsPort":443,"preferHTTPS":true,"pathPrefix":"/fleet"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/feeds", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/feeds", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index status=%d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	modes := payload["modes"].([]any)
	first := modes[0].(map[string]any)
	if path, _ := first["path"].(string); path != "/api/v1/feeds/all.xml" {
		t.Fatalf("path=%q", path)
	}
	url, _ := first["url"].(string)
	want := "https://feeds.example/fleet/api/v1/feeds/all.xml"
	if url != want {
		t.Fatalf("url=%q want %q", url, want)
	}
}
