package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequiresAuthPublicHooks(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/org.fleet.servarrbridge/secrettoken", nil)
	if requiresAuth(req) {
		t.Fatal("POST /api/v1/hooks/… should be public")
	}
	get := httptest.NewRequest(http.MethodGet, "/api/v1/hooks/org.fleet.servarrbridge/secrettoken", nil)
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
