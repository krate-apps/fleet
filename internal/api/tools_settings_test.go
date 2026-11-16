package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/tools"
)

func TestToolsSettingsGetPut(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	bin := filepath.Join(dir, "fake-ffmpeg")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := &tools.Runner{
		FfmpegPath:   "",
		AllowedRoots: []string{dir},
		ArtifactsDir: filepath.Join(dir, "artifacts"),
	}
	cfg := config.Default()
	cfg.Tools.Ffmpeg = ""
	cfg.DataDir = dir
	s := New(cfg, nil, nil, nil, Options{Store: store, Tools: runner})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/tools", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d %s", rec.Code, rec.Body.String())
	}

	body, _ := json.Marshal(map[string]any{
		"paths": tools.Paths{Ffmpeg: bin},
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/settings/tools", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status %d %s", rec.Code, rec.Body.String())
	}
	if runner.Paths().Ffmpeg != bin {
		t.Fatalf("runner not updated: %+v", runner.Paths())
	}

	raw, err := store.GetMeta(toolsSettingsMetaKey)
	if err != nil || raw == "" {
		t.Fatalf("meta missing: %v %q", err, raw)
	}
}
