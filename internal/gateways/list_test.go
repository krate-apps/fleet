package gateways

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListQBittorrent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	})
	mux.HandleFunc("/api/v2/torrents/info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"hash":        "aabbccdd",
			"name":        "Demo",
			"size":        1000,
			"downloaded":  1000,
			"uploaded":    200,
			"amount_left": 0,
			"dlspeed":     0,
			"upspeed":     10,
			"progress":    1,
			"ratio":       0.2,
			"state":       "uploading",
			"save_path":   "/data",
			"category":    "tv",
		}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rows, err := ListTorrents(context.Background(), ListInput{
		Kind: KindQBittorrent,
		URL:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Complete || rows[0].Name != "Demo" {
		t.Fatalf("%+v", rows)
	}
	mapped := rows[0].ToRTorrent()
	if mapped.Hash != "AABBCCDD" || mapped.State != "seeding" {
		t.Fatalf("%+v", mapped)
	}
}

func TestListTransmission(t *testing.T) {
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
				"torrents": []map[string]any{{
					"hashString":    "deadbeef",
					"name":          "Tx",
					"totalSize":     50,
					"downloadedEver": 25,
					"uploadedEver":  0,
					"leftUntilDone": 25,
					"rateDownload":  5,
					"rateUpload":    0,
					"uploadRatio":   0,
					"status":        4,
					"downloadDir":   "/dl",
					"percentDone":   0.5,
					"eta":           5,
					"peersConnected": 2,
				}},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rows, err := ListTorrents(context.Background(), ListInput{
		Kind: KindTransmission,
		URL:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Complete || !strings.EqualFold(rows[0].Hash, "deadbeef") {
		t.Fatalf("%+v", rows)
	}
}
