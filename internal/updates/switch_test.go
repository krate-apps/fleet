package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSwitchWithHealthRollback(t *testing.T) {
	prefix := t.TempDir()
	layout := Layout{Prefix: prefix}

	writeRelease := func(ver string) {
		dir := layout.ReleaseDir(ver)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fleet"), []byte("#!/bin/true\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeRelease("1.0.0")
	writeRelease("2.0.0")
	if _, err := layout.SwitchCurrent("1.0.0"); err != nil {
		t.Fatal(err)
	}
	cur, err := layout.ResolveCurrentVersion()
	if err != nil || cur != "1.0.0" {
		t.Fatalf("cur=%s err=%v", cur, err)
	}

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer fail.Close()

	res, err := layout.SwitchWithHealth(context.Background(), SwitchOptions{
		Version:    "2.0.0",
		HealthURL:  fail.URL,
		Timeout:    time.Second,
		HTTPClient: fail.Client(),
	})
	if err == nil {
		t.Fatal("expected health failure")
	}
	if res == nil || !res.RolledBack || res.Current != "1.0.0" {
		t.Fatalf("%+v", res)
	}
	cur, _ = layout.ResolveCurrentVersion()
	if cur != "1.0.0" {
		t.Fatalf("after rollback cur=%s", cur)
	}

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"fleet"}`))
	}))
	defer okSrv.Close()

	res, err = layout.SwitchWithHealth(context.Background(), SwitchOptions{
		Version:    "2.0.0",
		HealthURL:  okSrv.URL,
		HTTPClient: okSrv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RolledBack || res.Current != "2.0.0" || res.Previous != "1.0.0" {
		t.Fatalf("%+v", res)
	}
}
