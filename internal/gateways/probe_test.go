package gateways

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProbeQBittorrent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret" {
			_, _ = w.Write([]byte("Fails."))
			return
		}
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/app/version", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("v4.6.5"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := Probe(context.Background(), ProbeInput{
		Kind:     KindQBittorrent,
		URL:      srv.URL,
		Username: "admin",
		Password: "secret",
	})
	if !res.OK || res.Version != "v4.6.5" {
		t.Fatalf("unexpected probe result: %+v", res)
	}
}

func TestProbeDelugeIgnoresCookiePath(t *testing.T) {
	mux := http.NewServeMux()
	authed := false
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "auth.login":
			http.SetCookie(w, &http.Cookie{Name: "_session_id", Value: "abc123", Path: "/trevor/deluge/"})
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil, "id": req.ID})
		case "daemon.get_version", "daemon.info":
			if r.Header.Get("Cookie") == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"result": nil,
					"error":  map[string]any{"message": "Not authenticated", "code": 1},
					"id":     req.ID,
				})
				return
			}
			authed = true
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "2.1.1", "error": nil, "id": req.ID})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": nil,
				"error":  map[string]any{"message": "unexpected " + req.Method, "code": 2},
				"id":     req.ID,
			})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := Probe(context.Background(), ProbeInput{
		Kind:     KindDeluge,
		URL:      srv.URL,
		Password: "fleet-lab",
	})
	if !res.OK || res.Version != "2.1.1" || !authed {
		t.Fatalf("unexpected probe result: %+v authed=%v", res, authed)
	}
}

func TestProbeDelugeReconnectsDaemon(t *testing.T) {
	mux := http.NewServeMux()
	connected := false
	versionCalls := 0
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "auth.login":
			http.SetCookie(w, &http.Cookie{Name: "_session_id", Value: "abc", Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil, "id": req.ID})
		case "daemon.get_version", "daemon.info":
			versionCalls++
			if !connected {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"result": nil,
					"error":  map[string]any{"message": "Not connected", "code": 1},
					"id":     req.ID,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "2.2.0", "error": nil, "id": req.ID})
		case "web.connected":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": connected, "error": nil, "id": req.ID})
		case "web.get_hosts":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []any{[]any{"host-1", "127.0.0.1", 58846, "localclient"}},
				"error":  nil,
				"id":     req.ID,
			})
		case "web.connect":
			connected = true
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{}, "error": nil, "id": req.ID})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": nil,
				"error":  map[string]any{"message": "unexpected " + req.Method, "code": 2},
				"id":     req.ID,
			})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := Probe(context.Background(), ProbeInput{
		Kind:     KindDeluge,
		URL:      srv.URL,
		Password: "x",
	})
	if !res.OK || res.Version != "2.2.0" || !connected || versionCalls < 2 {
		t.Fatalf("unexpected probe result: %+v connected=%v versionCalls=%d", res, connected, versionCalls)
	}
}

func TestProbeDelugeConnectTimeoutMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "auth.login":
			http.SetCookie(w, &http.Cookie{Name: "_session_id", Value: "abc", Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil, "id": req.ID})
		case "daemon.get_version", "daemon.info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": nil,
				"error":  map[string]any{"message": "Not connected", "code": 1},
				"id":     req.ID,
			})
		case "web.connected":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": false, "error": nil, "id": req.ID})
		case "web.get_hosts":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []any{[]any{"host-1", "127.0.0.1", 58846, "localclient"}},
				"error":  nil,
				"id":     req.ID,
			})
		case "web.connect":
			time.Sleep(50 * time.Millisecond)
			// never responds before client timeout when using tiny context — but httptest still writes;
			// simulate hang by blocking on request context.
			<-r.Context().Done()
			return
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": nil, "error": nil, "id": req.ID})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	// Use a short-lived outer context; ensureDaemon's connect uses 12s child which inherits cancel.
	res := Probe(ctx, ProbeInput{Kind: KindDeluge, URL: srv.URL, Password: "x"})
	if res.OK {
		t.Fatalf("expected failure: %+v", res)
	}
	if !strings.Contains(res.Message, "daemon not connected") && !strings.Contains(res.Message, "timed out") {
		t.Fatalf("expected timeout/daemon message, got %q", res.Message)
	}
}

func TestProbeTransmissionSessionHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/transmission/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "abc")
			w.WriteHeader(http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"arguments": map[string]any{
				"version": "4.0.5",
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := Probe(context.Background(), ProbeInput{
		Kind: KindTransmission,
		URL:  srv.URL,
	})
	if !res.OK || !strings.Contains(res.Version, "4.0.5") {
		t.Fatalf("unexpected probe result: %+v", res)
	}
}
