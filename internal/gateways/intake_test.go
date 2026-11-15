package gateways

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntakeQBittorrentMagnet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/torrents/add", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		if got := r.FormValue("urls"); !strings.Contains(got, "magnet:?xt=urn:btih:abc") {
			t.Fatalf("urls=%q", got)
		}
		if r.FormValue("paused") != "false" {
			t.Fatalf("paused=%q", r.FormValue("paused"))
		}
		_, _ = w.Write([]byte("Ok."))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res := IntakeDownload(context.Background(), IntakeInput{
		Kind:    KindQBittorrent,
		URL:     srv.URL,
		Magnets: []string{"magnet:?xt=urn:btih:abc"},
		Start:   true,
	})
	if !res.OK {
		t.Fatalf("%+v", res)
	}
}

func TestIntakeTransmissionMagnet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/transmission/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "sid")
			w.WriteHeader(http.StatusConflict)
			return
		}
		var body struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Method != "torrent-add" {
			t.Fatalf("method=%s", body.Method)
		}
		if body.Arguments["filename"] != "magnet:?xt=urn:btih:abc" {
			t.Fatalf("%v", body.Arguments)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result":    "success",
			"arguments": map[string]any{"torrent-added": map[string]any{}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res := IntakeDownload(context.Background(), IntakeInput{
		Kind:    KindTransmission,
		URL:     srv.URL,
		Magnets: []string{"magnet:?xt=urn:btih:abc"},
		Start:   true,
	})
	if !res.OK {
		t.Fatalf("%+v", res)
	}
}
