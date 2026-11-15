package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/sync"
)

func TestSetNameWithoutClient(t *testing.T) {
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}))
	body, _ := json.Marshal(map[string]any{"name": "renamed"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/torrents/ABCDEF/name", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSetLabelWithoutClient(t *testing.T) {
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}))
	body, _ := json.Marshal(map[string]string{"label": "tv"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/torrents/ABCDEF/label", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSetTorrentPriorityValidation(t *testing.T) {
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}))
	body, _ := json.Marshal(map[string]any{"priority": 9})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/torrents/ABCDEF/priority", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected priority bounds 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCapabilitiesSignalsDisconnected(t *testing.T) {
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
		t.Fatalf("expected connected=false: %#v", body)
	}
	// CORE-017: UI must be able to read a diagnostic when the engine is down.
	if _, ok := body["message"]; !ok && body["capabilities"] == nil {
		t.Fatalf("expected message or capabilities field: %#v", body)
	}
}
