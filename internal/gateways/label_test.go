package gateways

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetQBittorrentCategory(t *testing.T) {
	mux := http.NewServeMux()
	created := false
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/torrents/createCategory", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "category=tv") {
			t.Fatalf("unexpected create form %q", body)
		}
		created = true
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/torrents/setCategory", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form := string(body)
		if !strings.Contains(form, "hashes=ABC123") || !strings.Contains(form, "category=tv") {
			t.Fatalf("unexpected form %q", form)
		}
		_, _ = w.Write([]byte("Ok."))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := SetTorrentLabel(context.Background(), LabelInput{
		Kind:     KindQBittorrent,
		URL:      srv.URL,
		Username: "u",
		Password: "p",
		Hash:     "abc123",
		Label:    "tv",
	})
	if !res.OK {
		t.Fatalf("set failed: %+v", res)
	}
	if !created {
		t.Fatal("expected createCategory before setCategory")
	}
}

func TestSetDelugeLabel(t *testing.T) {
	var calls []string
	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls = append(calls, req.Method)
		switch req.Method {
		case "auth.login":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil, "id": 1})
		case "daemon.get_version":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "2.1.1", "error": nil, "id": 10})
		case "label.add":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": nil, "error": nil, "id": 20})
		case "label.set_torrent":
			if len(req.Params) < 2 || req.Params[0] != "abc123def" || req.Params[1] != "movies" {
				t.Fatalf("unexpected set_torrent params %#v", req.Params)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": nil, "error": nil, "id": 21})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": nil, "error": map[string]any{"message": "unknown " + req.Method}, "id": 0})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := SetTorrentLabel(context.Background(), LabelInput{
		Kind:     KindDeluge,
		URL:      srv.URL,
		Password: "deluge",
		Hash:     "ABC123DEF",
		Label:    "movies",
	})
	if !res.OK {
		t.Fatalf("set failed: %+v calls=%v", res, calls)
	}
}

func TestSetTransmissionLabel(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/transmission/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "sid")
			w.WriteHeader(http.StatusConflict)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "success"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res := SetTorrentLabel(context.Background(), LabelInput{
		Kind:  KindTransmission,
		URL:   srv.URL,
		Hash:  "ABCDEF",
		Label: "tv",
	})
	if !res.OK {
		t.Fatalf("set failed: %+v", res)
	}
	args, _ := gotBody["arguments"].(map[string]any)
	if gotBody["method"] != "torrent-set" {
		t.Fatalf("method %#v", gotBody["method"])
	}
	ids, _ := args["ids"].([]any)
	labels, _ := args["labels"].([]any)
	if len(ids) != 1 || ids[0] != "abcdef" {
		t.Fatalf("ids %#v", ids)
	}
	if len(labels) != 1 || labels[0] != "tv" {
		t.Fatalf("labels %#v", labels)
	}

	res = SetTorrentLabel(context.Background(), LabelInput{
		Kind:  KindTransmission,
		URL:   srv.URL,
		Hash:  "ABCDEF",
		Label: "",
	})
	if !res.OK {
		t.Fatalf("clear failed: %+v", res)
	}
	args, _ = gotBody["arguments"].(map[string]any)
	labels, _ = args["labels"].([]any)
	if len(labels) != 0 {
		t.Fatalf("expected empty labels, got %#v", labels)
	}
}
