package logs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateUnit(t *testing.T) {
	ok, err := ValidateUnit("deluge@trevor")
	if err != nil || ok != "deluge@trevor" {
		t.Fatalf("got %q err=%v", ok, err)
	}
	ok, err = ValidateUnit("fleet@trevor.service")
	if err != nil || ok != "fleet@trevor" {
		t.Fatalf("strip .service: %q err=%v", ok, err)
	}
	if _, err := ValidateUnit("evil@trevor"); err == nil {
		t.Fatal("expected reject evil@")
	}
	if _, err := ValidateUnit("deluge@../../etc"); err == nil {
		t.Fatal("expected reject path-like instance")
	}
	if _, err := ValidateUnit("deluge;rm -rf /"); err == nil {
		t.Fatal("expected reject injection")
	}
}

func TestDefaultUnitForKind(t *testing.T) {
	if got := DefaultUnitForKind("Deluge", "trevor"); got != "deluge@trevor" {
		t.Fatalf("got %q", got)
	}
	if got := DefaultUnitForKind("flood", "trevor"); got != "" {
		t.Fatalf("unexpected %q", got)
	}
}

func TestRingSnapshotAndHandler(t *testing.T) {
	ring := NewRing(64)
	h := ring.Handler(slog.LevelDebug)
	logger := slog.New(h)
	logger.Info("hello", "k", "v")
	logger.Warn("careful")
	snap := ring.Snapshot(10)
	if len(snap) != 2 {
		t.Fatalf("len=%d", len(snap))
	}
	if snap[0].SourceID != "fleet" || snap[0].Message != "hello" || snap[0].Level != "info" {
		t.Fatalf("%+v", snap[0])
	}
	if snap[1].Level != "warn" {
		t.Fatalf("%+v", snap[1])
	}
}

func TestParseJournalJSON(t *testing.T) {
	raw := []byte(`{"MESSAGE":"started","PRIORITY":"6","__REALTIME_TIMESTAMP":"1700000000000000"}
{"MESSAGE":"boom","PRIORITY":"3","__REALTIME_TIMESTAMP":"1700000001000000"}
{"MESSAGE":"{\"time\":\"2026-11-12T01:23:43.525721837-05:00\",\"level\":\"INFO\",\"msg\":\"shutdown requested\"}","PRIORITY":"6","__REALTIME_TIMESTAMP":"1700000002000000"}
{"MESSAGE":"{\"time\":\"2026-11-12T01:23:43.591549087-05:00\",\"level\":\"INFO\",\"msg\":\"rtorrent configured\",\"address\":\"unix:///run/x.sock\"}","PRIORITY":"6","__REALTIME_TIMESTAMP":"1700000003000000"}
not-json
`)
	entries, err := parseJournalJSON(raw, "deluge@trevor")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("len=%d", len(entries))
	}
	if entries[0].SourceID != "journal:deluge@trevor" || entries[0].Level != "info" {
		t.Fatalf("%+v", entries[0])
	}
	if entries[1].Level != "error" || entries[1].Message != "boom" {
		t.Fatalf("%+v", entries[1])
	}
	if !entries[0].TS.Equal(time.UnixMicro(1700000000000000).UTC()) {
		t.Fatalf("ts=%v", entries[0].TS)
	}
	if entries[2].Message != "shutdown requested" || entries[2].Level != "info" {
		t.Fatalf("unwrap plain: %+v", entries[2])
	}
	if entries[3].Message != "rtorrent configured" || entries[3].Attrs["address"] != "unix:///run/x.sock" {
		t.Fatalf("unwrap attrs: %+v", entries[3])
	}
}

func TestReadJournalRejectsInvalidUnit(t *testing.T) {
	_, err := ReadJournal(context.Background(), "root@system", 10, func(context.Context, string, int) ([]byte, error) {
		t.Fatal("runner should not run")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAggregatorSourceFilter(t *testing.T) {
	ring := NewRing(64)
	_ = ring.Handler(slog.LevelInfo).Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "ring-line", 0))
	agg := NewAggregator(ring, Config{
		InstallPrefix:   "/opt/trevor/fleet",
		RTorrentAddress: "",
		Journal: func(_ context.Context, unit string, _ int) ([]byte, error) {
			return []byte(`{"MESSAGE":"j-` + unit + `","PRIORITY":"6","__REALTIME_TIMESTAMP":"1700000000000000"}` + "\n"), nil
		},
		Clients: []ClientRef{{
			ID: "gw_d", Kind: "deluge", Name: "Deluge lab", Enabled: true,
		}},
	})
	_, entries, err := agg.Query(context.Background(), Query{Source: "gateway:gw_d", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected deluge journal entries")
	}
	for _, e := range entries {
		if e.SourceID != "gateway:gw_d" {
			t.Fatalf("unexpected source %q msg=%q", e.SourceID, e.Message)
		}
	}
	_, fleetOnly, err := agg.Query(context.Background(), Query{Source: "fleet", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range fleetOnly {
		if e.SourceKind != "fleet" {
			t.Fatalf("fleet filter leaked %+v", e)
		}
	}
}

func TestValidateLogPathAndRead(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "app.log")
	if err := os.WriteFile(path, []byte("info ok\nERROR failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateLogPath("/etc/passwd", home); err == nil {
		t.Fatal("expected reject outside home")
	}
	got, err := ReadLogFile(path, home, "gateway:x", "deluge", "D", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Level != "error" {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got[0].Message, "info ok") {
		t.Fatalf("%+v", got[0])
	}
}

func TestQBittorrentFileFallback(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "qBittorrent", "qBittorrent", "data", "logs", "qbittorrent.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "(N) 2026-11-12T03:37:57 - Detected an attempt to add a duplicate torrent\n(W) 2026-11-12T03:38:01 - Something risky\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	agg := NewAggregator(NewRing(64), Config{
		HomeDir: home,
		Journal: func(context.Context, string, int) ([]byte, error) {
			return []byte{}, nil // empty journal like lab
		},
		Clients: []ClientRef{{
			ID: "gw_q", Kind: "qbittorrent", Name: "qBit", Enabled: true,
		}},
	})
	_, entries, err := agg.Query(context.Background(), Query{Source: "gateway:gw_q", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len=%d %+v", len(entries), entries)
	}
	if entries[0].Message != "Detected an attempt to add a duplicate torrent" || entries[0].Stream != "file" {
		t.Fatalf("%+v", entries[0])
	}
	if entries[1].Level != "warn" {
		t.Fatalf("%+v", entries[1])
	}
	src := agg.Sources(context.Background())
	found := false
	for _, s := range src {
		if s.ID == "gateway:gw_q" && strings.Contains(s.Detail, "file") {
			found = true
		}
	}
	if !found {
		t.Fatalf("sources=%+v", src)
	}
}

func TestInstanceUserFromPrefix(t *testing.T) {
	// Prefer current user when available; prefix fallback only when Current fails.
	// Just ensure DefaultUnit uses a non-empty username.
	u := InstanceUser("/opt/trevor/fleet")
	if u == "" {
		t.Fatal("empty user")
	}
}
