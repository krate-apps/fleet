package extensions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginStoreLoginUpsertsCookies(t *testing.T) {
	dir := t.TempDir()
	cookies := &CookieStore{DataDir: dir}
	store := &LoginStore{DataDir: dir, Cookies: cookies}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			http.SetCookie(w, &http.Cookie{Name: "csrf", Value: "tok"})
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/login":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("username") != "alice" || r.Form.Get("password") != "secret" {
				http.Error(w, "bad auth", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc"})
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := store.Set([]LoginAccount{{
		Name:      "demo",
		Origin:    srv.URL,
		Username:  "alice",
		Password:  "secret",
		LoginPath: "/login",
		WarmupGET: true,
		Enabled:   true,
	}}); err != nil {
		t.Fatal(err)
	}

	host, n, err := store.Login(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if host == "" || n < 1 {
		t.Fatalf("host=%q n=%d", host, n)
	}
	header, err := cookies.HeaderForURL(srv.URL + "/feed")
	if err != nil {
		t.Fatal(err)
	}
	if header == "" || !strings.Contains(header, "session=abc") {
		t.Fatalf("cookies=%q", header)
	}
}
