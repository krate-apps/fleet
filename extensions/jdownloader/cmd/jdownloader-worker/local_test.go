package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalJD_pingAndQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jdcheck.js", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("jdownloader=true;"))
	})
	mux.HandleFunc("/downloadsV2/queryPackages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"uuid":        7,
					"name":        "Local.Pkg",
					"status":      "Downloading",
					"bytesLoaded": 50,
					"bytesTotal":  100,
					"speed":       10,
					"eta":         5,
					"enabled":     true,
					"running":     true,
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cli, err := newLocalJD(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.ping(); err != nil {
		t.Fatal(err)
	}
	raw, err := cli.action("/downloadsV2/queryPackages", []any{map[string]any{"maxResults": -1}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := normalizeMyJDPackages(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Local.Pkg" || items[0].ID != "7" {
		t.Fatalf("%+v", items)
	}
}

func TestLocalJD_rejectsPublicURL(t *testing.T) {
	if _, err := newLocalJD("http://8.8.8.8:3128"); err == nil {
		t.Fatal("expected reject for public host")
	}
}
