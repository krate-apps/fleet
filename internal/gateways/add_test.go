package gateways

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAddQBittorrent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/torrents/add", func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			t.Fatalf("expected multipart, got %q", r.Header.Get("Content-Type"))
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		gotPath := ""
		gotSkip := ""
		gotFile := false
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(part)
			switch part.FormName() {
			case "savepath":
				gotPath = string(body)
			case "skip_checking":
				gotSkip = string(body)
			case "torrents":
				gotFile = len(body) > 0
			}
		}
		if gotPath != "/data/done" || gotSkip != "true" || !gotFile {
			t.Fatalf("unexpected form: path=%q skip=%q file=%v", gotPath, gotSkip, gotFile)
		}
		_, _ = w.Write([]byte("Ok."))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := AddTorrent(context.Background(), AddInput{
		Kind:     KindQBittorrent,
		URL:      srv.URL,
		Username: "u",
		Password: "p",
		Torrent:  []byte("d4:infod4:name1:aee"),
		SavePath: "/data/done",
	})
	if !res.OK {
		t.Fatalf("add failed: %+v", res)
	}
}

func TestAddTransmissionDuplicate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/transmission/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "sid")
			w.WriteHeader(http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"arguments": map[string]any{
				"torrent-duplicate": map[string]any{"id": 1},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := AddTorrent(context.Background(), AddInput{
		Kind:          KindTransmission,
		URL:           srv.URL,
		Torrent:       []byte("d4:infod4:name1:aee"),
		SavePath:      "/data/done",
		SkipIfPresent: true,
	})
	if !res.OK || !res.Skipped {
		t.Fatalf("expected skipped duplicate: %+v", res)
	}
}
