package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestSetupLoginLogoutAndLockout(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc, err := NewService(store, false)
	if err != nil {
		t.Fatal(err)
	}
	needs, err := svc.NeedsSetup()
	if err != nil || !needs {
		t.Fatalf("needs=%v err=%v", needs, err)
	}
	u, err := svc.CreateAdmin("alice", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAdmin("bob", "password123"); err == nil {
		t.Fatal("second setup should fail")
	}
	needs, _ = svc.NeedsSetup()
	if needs {
		t.Fatal("setup should be done")
	}
	got, err := svc.Authenticate("alice", "password123")
	if err != nil || got.ID != u.ID {
		t.Fatalf("%+v %v", got, err)
	}
	sess, err := svc.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	s2, user, err := svc.LookupSession(sess.ID)
	if err != nil || user.Username != "alice" || s2.CSRFToken != sess.CSRFToken {
		t.Fatalf("%+v %+v %v", s2, user, err)
	}
	rec := httptest.NewRecorder()
	svc.SetSessionCookies(rec, sess)
	if c := rec.Result().Cookies(); len(c) < 2 {
		t.Fatalf("cookies=%v", c)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", nil)
	req.Header.Set(CSRFHeader, sess.CSRFToken)
	if err := svc.ValidateCSRF(req, sess); err != nil {
		t.Fatal(err)
	}
	_ = svc.DeleteSession(sess.ID)
	if _, _, err := svc.LookupSession(sess.ID); err == nil {
		t.Fatal("expected logout")
	}

	key := "127.0.0.1:alice"
	for i := 0; i < maxFailures; i++ {
		svc.RecordLoginFailure(key)
	}
	if err := svc.CheckLoginAllowed(key); err == nil {
		t.Fatal("expected lockout")
	}
}

func TestAllowedWebSocketOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/v1/events/ws", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	if !AllowedWebSocketOrigin(req) {
		t.Fatal("same origin should pass")
	}
	req.Header.Set("Origin", "https://evil.example")
	if AllowedWebSocketOrigin(req) {
		t.Fatal("evil origin should fail")
	}
	req.Host = "host.example"
	req.Header.Set("Origin", "https://host.example.evil.com")
	if AllowedWebSocketOrigin(req) {
		t.Fatal("prefix-spoofed origin should fail")
	}
}
