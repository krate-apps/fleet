package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequiresAuthPublicHooks(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/org.fleet.servarrbridge", nil)
	if requiresAuth(req) {
		t.Fatal("POST /api/v1/hooks/… should be public")
	}
	get := httptest.NewRequest(http.MethodGet, "/api/v1/hooks/org.fleet.servarrbridge", nil)
	if !requiresAuth(get) {
		t.Fatal("GET hooks must still require auth")
	}
}

func TestHookIngestMethod(t *testing.T) {
	if got := hookIngestMethod("org.fleet.servarrbridge"); got != "ext.servarrbridge.ingest" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenMatches(t *testing.T) {
	if !tokenMatches("abc", "nope", "abc") {
		t.Fatal("expected match")
	}
	if tokenMatches("abc", "ab", "abcd") {
		t.Fatal("length mismatch must fail")
	}
	if tokenMatches("abc") {
		t.Fatal("empty candidates must fail")
	}
}

func TestHookHeaderToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/x", nil)
	if hookHeaderToken(req) != "" {
		t.Fatal("expected empty")
	}
	req.Header.Set("X-Api-Key", "k1")
	if got := hookHeaderToken(req); got != "k1" {
		t.Fatalf("api-key=%q", got)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/x", nil)
	req2.Header.Set("Authorization", "Bearer tok")
	if got := hookHeaderToken(req2); got != "tok" {
		t.Fatalf("bearer=%q", got)
	}
	// X-Api-Key wins when both present.
	req2.Header.Set("X-Api-Key", "prefer-key")
	if got := hookHeaderToken(req2); got != "prefer-key" {
		t.Fatalf("prefer=%q", got)
	}
}

func TestHandleExtensionHookUnauthorizedWithoutHeader(t *testing.T) {
	s := &Server{logger: slog.Default()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/org.fleet.servarrbridge", nil)
	req.SetPathValue("id", "org.fleet.servarrbridge")
	s.handleExtensionHook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}
