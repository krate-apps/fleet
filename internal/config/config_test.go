package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen.Port != 8080 {
		t.Fatalf("port = %d, want 8080", cfg.Listen.Port)
	}
	if cfg.Addr() != "127.0.0.1:8080" {
		t.Fatalf("Addr = %q", cfg.Addr())
	}
}

func TestLoadParsesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
listen:
  host: 0.0.0.0
  port: 9090
rtorrent:
  address: unix:///tmp/rtorrent.sock
log_level: debug
data_dir: /tmp/fleet-data
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen.Host != "0.0.0.0" || cfg.Listen.Port != 9090 {
		t.Fatalf("listen = %+v", cfg.Listen)
	}
	if cfg.RTorrent.Address != "unix:///tmp/rtorrent.sock" {
		t.Fatalf("rtorrent address = %q", cfg.RTorrent.Address)
	}
	if cfg.DataDir != "/tmp/fleet-data" {
		t.Fatalf("data_dir = %q", cfg.DataDir)
	}
}
