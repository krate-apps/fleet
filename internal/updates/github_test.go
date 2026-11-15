package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestListAvailableChannelFilter(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	mux := http.NewServeMux()
	var apiBase string
	mux.HandleFunc("/repos/krate-apps/fleet/releases", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
		  {
		    "tag_name": "v1.0.0",
		    "name": "Stable",
		    "prerelease": false,
		    "draft": false,
		    "published_at": "2026-01-01T00:00:00Z",
		    "body": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  fleet-v1.0.0-linux-amd64.tgz",
		    "assets": [
		      {"name": "fleet-v1.0.0-linux-amd64.tgz", "browser_download_url": "https://example.test/stable.tgz"}
		    ]
		  },
		  {
		    "tag_name": "v1.1.0-rc.1",
		    "name": "RC",
		    "prerelease": true,
		    "draft": false,
		    "published_at": "2026-02-01T00:00:00Z",
		    "body": "",
		    "assets": [
		      {"name": "fleet-v1.1.0-rc.1-linux-amd64.tgz", "browser_download_url": "https://example.test/pre.tgz"},
		      {"name": "CORE.SHA256", "browser_download_url": "` + apiBase + `/core.sha256"}
		    ]
		  },
		  {
		    "tag_name": "v0.0.1-draft",
		    "draft": true,
		    "prerelease": false,
		    "assets": []
		  }
		]`))
	})
	mux.HandleFunc("/core.sha256", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  fleet-v1.1.0-rc.1-linux-amd64.tgz\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	apiBase = srv.URL

	mgr := NewManager(store, dir, ManagerConfig{
		GitHubRepo:   "krate-apps/fleet",
		AssetPattern: "fleet-*-linux-amd64.tgz",
		APIBase:      srv.URL,
	})
	mgr.httpClient = srv.Client()

	if _, err := mgr.SetChannel(ChannelStable); err != nil {
		t.Fatal(err)
	}
	stable, err := mgr.ListAvailable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stable) != 1 || stable[0].Version != "v1.0.0" {
		t.Fatalf("stable=%+v", stable)
	}
	if stable[0].SHA256 == "" || stable[0].URL == "" {
		t.Fatalf("stable missing url/sha: %+v", stable[0])
	}

	if _, err := mgr.SetChannel(ChannelPre); err != nil {
		t.Fatal(err)
	}
	pre, err := mgr.ListAvailable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pre) != 2 {
		t.Fatalf("pre=%+v", pre)
	}
	foundRC := false
	for _, r := range pre {
		if r.Version == "v1.1.0-rc.1" {
			foundRC = true
			if r.SHA256 != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
				t.Fatalf("rc sha=%q", r.SHA256)
			}
		}
	}
	if !foundRC {
		t.Fatalf("missing rc in %+v", pre)
	}
}

func TestResolveManifestPrepareByVersion(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pkgDir := t.TempDir()
	archive := filepath.Join(pkgDir, "fleet.tar.gz")
	sum := writeArchive(t, archive, map[string]string{"bin/fleet": "#!/bin/true\n"})
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}

	pkgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(raw)
	}))
	defer pkgSrv.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/releases", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
		  {
		    "tag_name": "v9.9.9",
		    "prerelease": false,
		    "draft": false,
		    "body": "` + sum + `  fleet-v9.9.9-linux-amd64.tgz",
		    "assets": [
		      {"name": "fleet-v9.9.9-linux-amd64.tgz", "browser_download_url": "` + pkgSrv.URL + `/pkg.tgz"}
		    ]
		  }
		]`))
	})
	apiSrv := httptest.NewServer(mux)
	defer apiSrv.Close()

	mgr := NewManager(store, dir, ManagerConfig{
		GitHubRepo:   "owner/repo",
		AssetPattern: "fleet-*-linux-amd64.tgz",
		APIBase:      apiSrv.URL,
	})
	mgr.httpClient = &http.Client{Transport: dualHost{
		api: apiSrv.Client().Transport,
		pkg: pkgSrv.Client().Transport,
	}}

	pending, err := mgr.Prepare(context.Background(), Manifest{Version: "v9.9.9"})
	if err != nil {
		t.Fatal(err)
	}
	if pending.Version != "v9.9.9" {
		t.Fatalf("%+v", pending)
	}
}

type dualHost struct {
	api, pkg http.RoundTripper
}

func (d dualHost) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.URL.Path) >= 7 && req.URL.Path[:7] == "/repos/" {
		return d.api.RoundTrip(req)
	}
	return d.pkg.RoundTrip(req)
}

func TestListInstalledAndPlanSwitch(t *testing.T) {
	prefix := t.TempDir()
	data := t.TempDir()
	store, err := storage.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	layout := Layout{Prefix: prefix}
	for _, ver := range []string{"v0.1.11", "v0.1.12"} {
		dir := layout.ReleaseDir(ver)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fleet"), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := layout.SwitchCurrent("v0.1.11"); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(store, data, ManagerConfig{InstallPrefix: prefix})
	list, err := mgr.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("%+v", list)
	}
	var sawCurrent bool
	for _, r := range list {
		if r.Version == "v0.1.11" && r.Current && r.HasBinary {
			sawCurrent = true
		}
	}
	if !sawCurrent {
		t.Fatalf("current missing: %+v", list)
	}

	plan, err := mgr.PlanSwitch(context.Background(), "v0.1.12", PlanApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.PendingVersion != "v0.1.12" || plan.HelperCommand == "" || plan.BackupPath == "" {
		t.Fatalf("%+v", plan)
	}
	if !strings.Contains(plan.HelperCommand, "ln -sfn") || !strings.Contains(plan.HelperCommand, "v0.1.12") {
		t.Fatalf("helper command=%q", plan.HelperCommand)
	}
}

func TestParseChecksumText(t *testing.T) {
	sum := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	got := parseChecksumText(sum+"  fleet-v1.tgz\n", "fleet-v1.tgz")
	if got != sum {
		t.Fatalf("got %q", got)
	}
	got = parseChecksumText(sum+"\n", "anything.tgz")
	if got != sum {
		t.Fatalf("bare digest got %q", got)
	}
}
