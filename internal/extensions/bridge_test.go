package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/krate-apps/fleet/internal/jobs"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/tools"
	"github.com/krate-apps/fleet/sdk/backend"
)

type fakeFilesRoots struct {
	roots []string
}

func (f *fakeFilesRoots) Roots() []string { return f.roots }
func (f *fakeFilesRoots) List(string) ([]map[string]any, string, error) {
	return nil, "", nil
}
func (f *fakeFilesRoots) Mkdir(string) error                          { return nil }
func (f *fakeFilesRoots) Rename(string, string) error                 { return nil }
func (f *fakeFilesRoots) Remove(string, bool) error                   { return nil }
func (f *fakeFilesRoots) Copy(string, string) error                   { return nil }
func (f *fakeFilesRoots) Move(string, string) error                   { return nil }
func (f *fakeFilesRoots) ReadText(string) (string, error)             { return "", nil }
func (f *fakeFilesRoots) ReadBase64(string) (string, int64, error)    { return "", 0, nil }
func (f *fakeFilesRoots) Checksum(string) (string, string, int64, error) {
	return "", "", 0, nil
}
func (f *fakeFilesRoots) Stat(string) (map[string]any, error) {
	return map[string]any{"path": "", "size": 0, "device": 1, "inode": 1, "nlink": 1, "isDir": false}, nil
}
func (f *fakeFilesRoots) Fingerprint(string, int) (int64, string, error) {
	return 0, "", nil
}
func (f *fakeFilesRoots) Hardlink(string, string) error { return nil }

type fakeTorrents struct {
	magnets []string
	labels  map[string]string
	actions []string
}

func (f *fakeTorrents) AddMagnets(_ context.Context, magnets []string, _ bool) error {
	f.magnets = append(f.magnets, magnets...)
	return nil
}

func (f *fakeTorrents) AddTorrentFile(_ context.Context, _ []byte, _ bool) error {
	return nil
}

func (f *fakeTorrents) TorrentMetainfo(_ context.Context, _ string) (string, string, error) {
	return "https://rutracker.org/forum/viewtopic.php?t=1", "https://bt.t-ru.org/ann", nil
}

func (f *fakeTorrents) ListExtTorrents(_ context.Context, allClients bool) ([]ExtTorrent, error) {
	primary := ExtTorrent{Hash: "ABC", Name: "n", Ratio: 1.5, Label: "seed", State: "seeding", UpTotal: 100, Complete: true, ClientID: "local-rtorrent"}
	if !allClients {
		return []ExtTorrent{primary}, nil
	}
	return []ExtTorrent{
		primary,
		{Hash: "DEF", Name: "gw", State: "error", ClientID: "gw-1"},
	}, nil
}

func (f *fakeTorrents) ApplyExtAction(_ context.Context, action string, hashes []string) error {
	f.actions = append(f.actions, action+":"+hashes[0])
	return nil
}

func (f *fakeTorrents) SetLabel(_ context.Context, hash, label string) error {
	if f.labels == nil {
		f.labels = map[string]string{}
	}
	f.labels[hash] = label
	return nil
}

func (f *fakeTorrents) SetRates(_ context.Context, _, _ string, _, _ int64) error {
	return nil
}

func (f *fakeTorrents) UnpackArchive(_ context.Context, _, _ string) (int, error) {
	return 1, nil
}

func (f *fakeTorrents) AssignThrottles(_ context.Context, _ []string, _ string) error {
	return nil
}

func (f *fakeTorrents) MediaInfo(_ context.Context, _ string, _ int, _ string) (string, error) {
	return "General\nComplete name: test.mkv\n", nil
}

func (f *fakeTorrents) Screenshots(_ context.Context, _ string, _ int, _ string, _, _, _, _ int) ([]map[string]any, error) {
	return []map[string]any{{"index": 0, "dataUrl": "data:image/jpeg;base64,xx", "bytes": 2}}, nil
}

func (f *fakeTorrents) Spectrogram(_ context.Context, _ string, _ int, _, _ string, _ int) (map[string]any, error) {
	return map[string]any{"dataUrl": "data:image/png;base64,xx", "bytes": 2, "mime": "image/png"}, nil
}

func (f *fakeTorrents) ListTrackers(_ context.Context, _ string) ([]ExtTracker, error) {
	return []ExtTracker{{Index: 0, ID: "ABC:t0", URL: "https://tracker.example/announce", Enabled: true}}, nil
}

func (f *fakeTorrents) AddTracker(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeTorrents) ListPeers(_ context.Context, _ string) ([]ExtPeer, error) {
	return []ExtPeer{{ID: "ABC:peer1", Address: "203.0.113.10", ClientVersion: "test/1"}}, nil
}

func (f *fakeTorrents) DisconnectPeer(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeTorrents) SetDirectory(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeTorrents) ResolveSource(_ context.Context, hash string) (string, []byte, error) {
	return hash + ".torrent", []byte("d4:infod4:name1:aee"), nil
}

func TestCoreBridgeStorageAndTorrents(t *testing.T) {
	dir := t.TempDir()
	bridge := &CoreBridge{
		Storage:  &StorageBridge{DataDir: dir},
		Torrents: &fakeTorrents{},
	}
	caps := []string{"storage.read", "storage.write", "torrents.add", "torrents.read", "torrents.actions", "files.unpack", "files.mediainfo", "trackers.read", "trackers.write", "peers.read", "peers.actions"}
	_, err := bridge.Handle(context.Background(), "org.fleet.rssfeeds", caps, backend.MethodStoragePut, json.RawMessage(`{"key":"feeds","value":{"feeds":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := bridge.Handle(context.Background(), "org.fleet.rssfeeds", caps, backend.MethodStorageGet, json.RawMessage(`{"key":"feeds"}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := got.(json.RawMessage)
	if !ok || string(raw) == "null" {
		t.Fatalf("got %#v", got)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.rssfeeds", caps, backend.MethodTorrentsAdd, json.RawMessage(`{"magnets":["magnet:?xt=urn:btih:abc"]}`))
	if err != nil {
		t.Fatal(err)
	}
	ft := bridge.Torrents.(*fakeTorrents)
	if len(ft.magnets) != 1 {
		t.Fatalf("magnets=%v", ft.magnets)
	}
	list, err := bridge.Handle(context.Background(), "org.fleet.ratio", caps, backend.MethodTorrentsList, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	m := list.(map[string]any)
	if m["torrents"] == nil {
		t.Fatalf("%#v", list)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.ratio", caps, backend.MethodTorrentsAction, json.RawMessage(`{"action":"stop","hashes":["ABC"]}`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.ratio", caps, backend.MethodTorrentsLabel, json.RawMessage(`{"hash":"ABC","label":"rat1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if ft.labels["ABC"] != "rat1" {
		t.Fatalf("labels=%v", ft.labels)
	}
	un, err := bridge.Handle(context.Background(), "org.fleet.archiveextract", caps, backend.MethodFilesUnpack, json.RawMessage(`{"archive":"/data/a.zip","dest":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if un.(map[string]any)["extracted"].(int) != 1 {
		t.Fatalf("%#v", un)
	}
	mi, err := bridge.Handle(context.Background(), "org.fleet.mediainfo", caps, backend.MethodFilesMediainfo, json.RawMessage(`{"path":"/data/test.mkv"}`))
	if err != nil {
		t.Fatal(err)
	}
	if mi.(map[string]any)["text"].(string) == "" {
		t.Fatalf("%#v", mi)
	}
	tr, err := bridge.Handle(context.Background(), "org.fleet.trackerreplace", caps, backend.MethodTrackersList, json.RawMessage(`{"hash":"ABC"}`))
	if err != nil {
		t.Fatal(err)
	}
	if tr.(map[string]any)["trackers"] == nil {
		t.Fatalf("%#v", tr)
	}
	peers, err := bridge.Handle(context.Background(), "org.fleet.blocklist", caps, backend.MethodPeersList, json.RawMessage(`{"hash":"ABC"}`))
	if err != nil {
		t.Fatal(err)
	}
	if peers.(map[string]any)["peers"] == nil {
		t.Fatalf("%#v", peers)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.blocklist", caps, backend.MethodPeersDisconnect, json.RawMessage(`{"hash":"ABC","address":"203.0.113.10"}`))
	if err != nil {
		t.Fatal(err)
	}
	add, err := bridge.Handle(context.Background(), "org.fleet.trackerreplace", caps, backend.MethodTrackersAdd, json.RawMessage(`{"hash":"ABC","url":"https://extra.example/announce"}`))
	if err != nil {
		t.Fatal(err)
	}
	if add.(map[string]any)["added"].(int) != 1 {
		t.Fatalf("%#v", add)
	}
}

func TestCoreBridgeURLRewrite(t *testing.T) {
	dir := t.TempDir()
	bridge := &CoreBridge{URLRewrite: &URLRewriteStore{DataDir: dir}}
	caps := []string{"urlrewrite.read", "urlrewrite.write"}
	_, err := bridge.Handle(context.Background(), "org.fleet.rssfeedsrewrite", caps, backend.MethodURLRewriteSet, json.RawMessage(`{"rules":[{"name":"https","pattern":"^http://","replacement":"https://","enabled":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	list, err := bridge.Handle(context.Background(), "org.fleet.rssfeedsrewrite", caps, backend.MethodURLRewriteList, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if list.(map[string]any)["rules"] == nil {
		t.Fatalf("%#v", list)
	}
	applied, err := bridge.Handle(context.Background(), "org.fleet.rssfeeds", []string{"urlrewrite.read"}, backend.MethodURLRewriteApply, json.RawMessage(`{"url":"http://x.example/a"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := applied.(map[string]any)
	if m["url"] != "https://x.example/a" || m["changed"] != true || m["matchedRule"] != "https" {
		t.Fatalf("%#v", applied)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.rssfeeds", nil, backend.MethodURLRewriteApply, json.RawMessage(`{"url":"http://x"}`))
	if err == nil {
		t.Fatal("expected capability error")
	}
}

func TestCoreBridgeRejectsMissingCapability(t *testing.T) {
	bridge := &CoreBridge{Storage: &StorageBridge{DataDir: t.TempDir()}}
	_, err := bridge.Handle(context.Background(), "org.fleet.rssfeeds", nil, backend.MethodStorageGet, json.RawMessage(`{"key":"feeds"}`))
	if err == nil {
		t.Fatal("expected capability error")
	}
}

func TestCoreBridgeCookies(t *testing.T) {
	dir := t.TempDir()
	bridge := &CoreBridge{Cookies: &CookieStore{DataDir: dir}}
	caps := []string{"cookies.read", "cookies.write"}
	_, err := bridge.Handle(context.Background(), "org.fleet.cookies", caps, backend.MethodCookiesSet, json.RawMessage(`{"hosts":[{"host":"ex.ample","cookies":"a=1; b=2"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	list, err := bridge.Handle(context.Background(), "org.fleet.cookies", caps, backend.MethodCookiesList, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if list.(map[string]any)["hosts"] == nil {
		t.Fatalf("%#v", list)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.cookies", nil, backend.MethodCookiesList, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected capability error")
	}
}

func TestCoreBridgeTorrentsListAllClients(t *testing.T) {
	bridge := &CoreBridge{Torrents: &fakeTorrents{}}
	caps := []string{"torrents.read"}
	primary, err := bridge.Handle(context.Background(), "org.fleet.diskcleanup", caps, backend.MethodTorrentsList, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(primary.(map[string]any)["torrents"].([]ExtTorrent)); n != 1 {
		t.Fatalf("primary count=%d", n)
	}
	all, err := bridge.Handle(context.Background(), "org.fleet.diskcleanup", caps, backend.MethodTorrentsList, json.RawMessage(`{"allClients":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(all.(map[string]any)["torrents"].([]ExtTorrent)); n != 2 {
		t.Fatalf("allClients count=%d", n)
	}
}

func TestCoreBridgeJobsListAndSystemStatus(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	jm := jobs.NewManager(store)
	_, err = jm.Create(jobs.CreateInput{Kind: "rclone.copy", ExtensionID: "org.fleet.rclone"})
	if err != nil {
		t.Fatal(err)
	}
	bridge := &CoreBridge{
		Jobs:  jm,
		Files: &fakeFilesRoots{roots: []string{t.TempDir()}},
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.metrics", nil, backend.MethodJobsList, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected jobs.read capability error")
	}
	list, err := bridge.Handle(context.Background(), "org.fleet.metrics", []string{"jobs.read"}, backend.MethodJobsList, json.RawMessage(`{"limit":10}`))
	if err != nil {
		t.Fatal(err)
	}
	jobsOut := list.(map[string]any)["jobs"].([]jobs.Job)
	if len(jobsOut) != 1 || jobsOut[0].Kind != "rclone.copy" {
		t.Fatalf("%#v", list)
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.metrics", nil, backend.MethodSystemStatus, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected system.read capability error")
	}
	st, err := bridge.Handle(context.Background(), "org.fleet.metrics", []string{"system.read"}, backend.MethodSystemStatus, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	m := st.(map[string]any)
	if m["disks"] == nil {
		t.Fatalf("%#v", st)
	}

	bridge.Engine = &fakeEngine{caps: map[string]any{"connected": true, "listenPort": int64(51413)}}
	_, err = bridge.Handle(context.Background(), "org.fleet.portwatch", nil, backend.MethodEngineCapabilities, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected engine.capabilities capability error")
	}
	eng, err := bridge.Handle(context.Background(), "org.fleet.portwatch", []string{"engine.capabilities"}, backend.MethodEngineCapabilities, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	em := eng.(map[string]any)
	if em["listenPort"] != int64(51413) || em["connected"] != true {
		t.Fatalf("%#v", eng)
	}
}

type fakeEngine struct {
	caps map[string]any
}

func (f *fakeEngine) Capabilities() map[string]any {
	return f.caps
}

func TestCoreBridgeToolsMktorrent(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	jm := jobs.NewManager(store)
	root := t.TempDir()
	src := filepath.Join(root, "payload.bin")
	if err := os.WriteFile(src, []byte("hello-nzb"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "mktorrent")
	script := `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then out="$2"; shift 2; continue; fi
  shift
done
printf 'd4:infod4:name4:demoee' > "$out"
echo '{"ok":true,"json":true}'
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	bridge := &CoreBridge{
		Jobs:    jm,
		DataDir: dataDir,
		Tools: &tools.Runner{
			MktorrentPath: bin,
			AllowedRoots:  []string{root},
			ArtifactsDir:  filepath.Join(dataDir, "artifacts"),
		},
	}
	_, err = bridge.Handle(context.Background(), "org.fleet.nzbbridge", nil, backend.MethodToolsMktorrent, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected tools.mktorrent capability error")
	}
	params, _ := json.Marshal(map[string]any{
		"paths":    []string{src},
		"announce": []string{"https://tracker.example/announce"},
		"private":  true,
		"name":     "demo",
	})
	out, err := bridge.Handle(context.Background(), "org.fleet.nzbbridge", []string{"tools.mktorrent"}, backend.MethodToolsMktorrent, params)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	jobID, _ := m["jobId"].(string)
	if jobID == "" {
		t.Fatalf("%#v", out)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, err := jm.Get(jobID)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status == jobs.StatusSucceeded || j.Status == jobs.StatusFailed {
			if j.Status != jobs.StatusSucceeded {
				t.Fatalf("job failed: %s", j.Error)
			}
			if j.ExtensionID != "org.fleet.nzbbridge" {
				t.Fatalf("extensionId=%q", j.ExtensionID)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout status=%s", j.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
