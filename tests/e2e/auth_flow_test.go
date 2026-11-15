package e2e_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/krate-apps/fleet/internal/api"
	"github.com/krate-apps/fleet/internal/auth"
	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/sync"
)

// In-process e2e covering setup → login → authenticated torrents (CDC exit #8).
func TestAuthSetupAndSession(t *testing.T) {
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
	srv := api.New(config.Default(), nil, nil, sync.NewEngine(nil, nil, sync.Options{}), api.Options{Auth: authSvc})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	res, err := client.Get(ts.URL + "/api/v1/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var st map[string]any
	_ = json.NewDecoder(res.Body).Decode(&st)
	if st["needsSetup"] != true {
		t.Fatalf("%+v", st)
	}

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "password123"})
	res, err = client.Post(ts.URL+"/api/v1/auth/setup", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("setup %d", res.StatusCode)
	}

	u, _ := url.Parse(ts.URL)
	var csrf string
	for _, c := range jar.Cookies(u) {
		if c.Name == auth.CSRFCookie {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("missing csrf cookie")
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/torrents", nil)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("torrents %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/torrents/actions", bytes.NewReader([]byte(`{"action":"start","hashes":["x"]}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, csrf)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		t.Fatalf("csrf+session should authenticate, got %d", res.StatusCode)
	}
}
