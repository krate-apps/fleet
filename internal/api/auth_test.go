package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/krate-apps/fleet/internal/auth"
	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/sync"
)

func TestAuthSetupLoginProtectsAPI(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	authSvc, err := auth.NewService(store, false)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}), Options{Auth: authSvc})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/torrents", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("before setup status=%d", rec.Code)
	}

	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "password123"})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/torrents", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed torrents status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/torrents/actions", bytes.NewReader([]byte(`{"action":"start","hashes":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected csrf fail status=%d", rec.Code)
	}

	var csrf string
	for _, c := range cookies {
		if c.Name == auth.CSRFCookie {
			csrf = c.Value
		}
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/torrents/actions", bytes.NewReader([]byte(`{"action":"start","hashes":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, csrf)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	srv.Handler().ServeHTTP(rec, req)
	// CSRF passed; handler may still reject empty hashes with 400.
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("csrf should pass status=%d body=%s", rec.Code, rec.Body.String())
	}
}
