package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/logs"
	"github.com/krate-apps/fleet/internal/sync"
)

func TestHealth(t *testing.T) {
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %#v", body)
	}
}

func TestVersion(t *testing.T) {
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTorrentsSnapshotEmpty(t *testing.T) {
	eng := sync.NewEngine(nil, nil, sync.Options{})
	srv := New(config.Default(), nil, nil, eng)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/torrents", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["torrents"]; !ok {
		t.Fatalf("body=%#v", body)
	}
}

func TestCapabilitiesWithoutClient(t *testing.T) {
	eng := sync.NewEngine(nil, nil, sync.Options{})
	srv := New(config.Default(), nil, nil, eng)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engine/capabilities", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["connected"] != false {
		t.Fatalf("body=%#v", body)
	}
}

func TestLogsEndpoint(t *testing.T) {
	ring := logs.NewRing(64)
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}), Options{
		Logs: logs.NewAggregator(ring, logs.Config{
			Journal: func(_ context.Context, _ string, _ int) ([]byte, error) {
				return []byte{}, nil
			},
		}),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs?limit=50", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["sources"]; !ok {
		t.Fatalf("body=%#v", body)
	}
	if _, ok := body["entries"]; !ok {
		t.Fatalf("body=%#v", body)
	}
}

func TestLogsUnavailable(t *testing.T) {
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}
