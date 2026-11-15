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

func TestTorrentActionsValidation(t *testing.T) {
	eng := sync.NewEngine(nil, nil, sync.Options{})
	srv := New(config.Default(), nil, nil, eng)

	body, _ := json.Marshal(map[string]any{"action": "start", "hashes": []string{"ABC"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/torrents/actions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	body, _ = json.Marshal(map[string]any{"action": "explode", "hashes": []string{"ABC"}})
	cfg := config.Default()
	// Still no client — unsupported action checked before client use... actually
	// unsupported is checked first. Re-run with engine that has no client after fixing:
	req = httptest.NewRequest(http.MethodPost, "/api/v1/torrents/actions", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	New(cfg, nil, nil, sync.NewEngine(nil, nil, sync.Options{})).Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAddTorrentsRequiresPayload(t *testing.T) {
	eng := sync.NewEngine(nil, nil, sync.Options{})
	srv := New(config.Default(), nil, nil, eng)
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/torrents", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	// no client → 503 before payload check... actually withClient fails after decode.
	// Order: decode, then magnets/torrent check, then engine call.
	// Empty payload → 400
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
