package extensions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/jobs"
	"github.com/krate-apps/fleet/internal/sysinfo"
	"github.com/krate-apps/fleet/internal/tools"
	"github.com/krate-apps/fleet/sdk/backend"
)

// ExtTorrent is the public torrent snapshot visible to workers (torrents.read).
type ExtTorrent struct {
	Hash         string  `json:"hash"`
	Name         string  `json:"name"`
	Ratio        float64 `json:"ratio"`
	Label        string  `json:"label,omitempty"`
	State        string  `json:"state"`
	SizeBytes    int64   `json:"sizeBytes"`
	DownTotal    int64   `json:"downTotal"`
	UpTotal      int64   `json:"upTotal"`
	DownRate     int64   `json:"downRate,omitempty"`
	UpRate       int64   `json:"upRate,omitempty"`
	Complete     bool    `json:"complete"`
	FinishedAt   int64   `json:"finishedAt,omitempty"`
	Directory    string  `json:"directory,omitempty"`
	ThrottleName string  `json:"throttleName,omitempty"`
	Private      bool    `json:"private,omitempty"`
	ClientID     string  `json:"clientId,omitempty"`
}

// ExtTracker is a tracker row visible to workers (trackers.read).
type ExtTracker struct {
	Index   int64  `json:"index"`
	ID      string `json:"id"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// ExtPeer is a connected peer visible to workers (peers.read).
type ExtPeer struct {
	ID            string `json:"id,omitempty"`
	Address       string `json:"address"`
	ClientVersion string `json:"clientVersion,omitempty"`
}

// TorrentsControl is the public torrent surface workers may invoke.
type TorrentsControl interface {
	AddMagnets(ctx context.Context, magnets []string, start bool) error
	AddTorrentFile(ctx context.Context, data []byte, start bool) error
	ListExtTorrents(ctx context.Context, allClients bool) ([]ExtTorrent, error)
	ApplyExtAction(ctx context.Context, action string, hashes []string) error
	SetLabel(ctx context.Context, hash, label string) error
	SetRates(ctx context.Context, hash, clientID string, upBps, downBps int64) error
	UnpackArchive(ctx context.Context, archivePath, destDir string) (extracted int, err error)
	AssignThrottles(ctx context.Context, hashes []string, throttleName string) error
	MediaInfo(ctx context.Context, hash string, index int, path string) (text string, err error)
	Screenshots(ctx context.Context, hash string, index int, path string, count, offset, interval, width int) (frames []map[string]any, err error)
	Spectrogram(ctx context.Context, hash string, index int, path, title string, width int) (image map[string]any, err error)
	ListTrackers(ctx context.Context, hash string) ([]ExtTracker, error)
	AddTracker(ctx context.Context, hash, trackerURL string) error
	ListPeers(ctx context.Context, hash string) ([]ExtPeer, error)
	DisconnectPeer(ctx context.Context, hash, address string) error
	SetDirectory(ctx context.Context, hash, directory string) error
	TorrentMetainfo(ctx context.Context, hash string) (comment, announce string, err error)
	// ResolveSource returns raw .torrent bytes for hash (session/tied file).
	ResolveSource(ctx context.Context, hash string) (filename string, data []byte, err error)
}

// ExtGateway is a registered client endpoint visible to workers (gateways.read).
type ExtGateway struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// GatewaysControl exposes Settings → Clients inventory to workers.
type GatewaysControl interface {
	ListGateways(ctx context.Context) ([]ExtGateway, error)
	AddTorrent(ctx context.Context, gatewayID string, filename string, torrent []byte, savePath string, skipIfPresent bool) (ok bool, skipped bool, message string, err error)
	SetTorrentLabel(ctx context.Context, gatewayID, hash, label string) error
}

// IntakeControl routes magnets/.torrent adds through the default (or explicit) client.
type IntakeControl interface {
	Add(ctx context.Context, magnets []string, torrent []byte, filename string, start bool, gatewayID string) (resolvedGatewayID string, err error)
}

// ThrottleControl exposes global and named bandwidth caps for extensions.
type ThrottleControl interface {
	GetGlobalRates(ctx context.Context) (downRate int64, upRate int64, err error)
	SetGlobalRates(ctx context.Context, downRate, upRate int64) error
	EnsureNamed(ctx context.Context, name string, upRate, downRate int64) error
}

// FilesControl is the path-guarded filesystem surface for workers (files.read/write).
type FilesControl interface {
	Roots() []string
	List(path string) (entries []map[string]any, cwd string, err error)
	Mkdir(path string) error
	Rename(from, to string) error
	Remove(path string, recursive bool) error
	Copy(from, to string) error
	Move(from, to string) error
	ReadText(path string) (string, error)
	ReadBase64(path string) (dataURL string, size int64, err error)
	Checksum(path string) (algo, hex string, size int64, err error)
	Stat(path string) (map[string]any, error)
	Fingerprint(path string, nBytes int) (size int64, digest string, err error)
	Hardlink(from, to string) error
}

// EngineControl exposes primary engine status (listen port, connected) to workers.
type EngineControl interface {
	Capabilities() map[string]any
}

// CoreBridge implements HostBridge for storage, HTTP fetch, torrents, and throttle.
type CoreBridge struct {
	Storage    *StorageBridge
	Torrents   TorrentsControl
	Gateways   GatewaysControl
	Intake     IntakeControl
	Jobs       *jobs.Manager
	Throttle   ThrottleControl
	Files      FilesControl
	Engine     EngineControl
	URLRewrite *URLRewriteStore
	Cookies    *CookieStore
	LoginMgr   *LoginStore
	HTTP       *http.Client
	// Tools is the shared helper runner template (mktorrent path + download roots).
	Tools *tools.Runner
	// DataDir is Fleet's data directory (job artifacts live under artifacts/<jobId>).
	DataDir string
}

// Handle dispatches reverse RPC methods with capability checks.
func (b *CoreBridge) Handle(ctx context.Context, extensionID string, caps []string, method string, params json.RawMessage) (any, error) {
	switch method {
	case backend.MethodStorageGet:
		if !HasCap(caps, "storage.read") {
			return nil, fmt.Errorf("capability storage.read required")
		}
		var p struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return b.Storage.Get(extensionID, p.Key)
	case backend.MethodStoragePut:
		if !HasCap(caps, "storage.write") {
			return nil, fmt.Errorf("capability storage.write required")
		}
		var p struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Storage.Put(extensionID, p.Key, p.Value); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case backend.MethodJobsCreate:
		if !HasCap(caps, "jobs.schedule") {
			return nil, fmt.Errorf("capability jobs.schedule required")
		}
		if b.Jobs == nil {
			return nil, fmt.Errorf("jobs unavailable")
		}
		var p struct {
			Kind    string `json:"kind"`
			Payload any    `json:"payload"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		j, err := b.Jobs.Create(jobs.CreateInput{
			Kind:        p.Kind,
			ExtensionID: extensionID,
			Payload:     p.Payload,
		})
		if err != nil {
			return nil, err
		}
		return j, nil
	case backend.MethodToolsMktorrent:
		if !HasCap(caps, "tools.mktorrent") {
			return nil, fmt.Errorf("capability tools.mktorrent required")
		}
		if b.Jobs == nil {
			return nil, fmt.Errorf("jobs unavailable")
		}
		if b.Tools == nil {
			return nil, fmt.Errorf("tools runner unavailable")
		}
		var p struct {
			Paths       []string `json:"paths"`
			Announce    []string `json:"announce"`
			PieceLength int      `json:"pieceLength"`
			Private     bool     `json:"private"`
			Name        string   `json:"name"`
			Comment     string   `json:"comment"`
			OutputName  string   `json:"outputName"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		job, err := tools.StartMktorrentJob(b.Jobs, b.Tools, b.DataDir, extensionID, tools.CreateInput{
			Paths:       p.Paths,
			Announce:    p.Announce,
			PieceLength: p.PieceLength,
			Private:     p.Private,
			Name:        p.Name,
			Comment:     p.Comment,
			OutputName:  p.OutputName,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "jobId": job.ID, "job": job}, nil
	case backend.MethodJobsUpdate:
		if !HasCap(caps, "jobs.schedule") {
			return nil, fmt.Errorf("capability jobs.schedule required")
		}
		if b.Jobs == nil {
			return nil, fmt.Errorf("jobs unavailable")
		}
		var p struct {
			ID       string  `json:"id"`
			Progress *float64 `json:"progress"`
			Message  string  `json:"message"`
			Status   string  `json:"status"`
			Error    string  `json:"error"`
			Result   any     `json:"result"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		j, err := b.Jobs.Get(strings.TrimSpace(p.ID))
		if err != nil {
			return nil, err
		}
		if j.ExtensionID != extensionID {
			return nil, fmt.Errorf("job not owned by extension")
		}
		status := strings.ToLower(strings.TrimSpace(p.Status))
		switch status {
		case jobs.StatusSucceeded:
			if err := b.Jobs.Succeed(j.ID, p.Result); err != nil {
				return nil, err
			}
		case jobs.StatusFailed:
			msg := strings.TrimSpace(p.Error)
			if msg == "" {
				msg = strings.TrimSpace(p.Message)
			}
			if msg == "" {
				msg = "failed"
			}
			if err := b.Jobs.Fail(j.ID, msg); err != nil {
				return nil, err
			}
		case jobs.StatusRunning, "":
			if p.Progress != nil {
				if err := b.Jobs.UpdateProgress(j.ID, *p.Progress, p.Message); err != nil {
					return nil, err
				}
			} else if err := b.Jobs.MarkRunning(j.ID, p.Message); err != nil {
				return nil, err
			}
		case jobs.StatusCancelled:
			if _, err := b.Jobs.Cancel(j.ID); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported status %q", status)
		}
		out, err := b.Jobs.Get(j.ID)
		if err != nil {
			return nil, err
		}
		return out, nil
	case backend.MethodJobsGet:
		if !HasCap(caps, "jobs.read") && !HasCap(caps, "jobs.schedule") {
			return nil, fmt.Errorf("capability jobs.read or jobs.schedule required")
		}
		if b.Jobs == nil {
			return nil, fmt.Errorf("jobs unavailable")
		}
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		j, err := b.Jobs.Get(strings.TrimSpace(p.ID))
		if err != nil {
			return nil, err
		}
		if j.ExtensionID != extensionID {
			return nil, fmt.Errorf("job not owned by extension")
		}
		return j, nil
	case backend.MethodJobsCancel:
		if !HasCap(caps, "jobs.schedule") {
			return nil, fmt.Errorf("capability jobs.schedule required")
		}
		if b.Jobs == nil {
			return nil, fmt.Errorf("jobs unavailable")
		}
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		j, err := b.Jobs.Get(strings.TrimSpace(p.ID))
		if err != nil {
			return nil, err
		}
		if j.ExtensionID != extensionID {
			return nil, fmt.Errorf("job not owned by extension")
		}
		out, err := b.Jobs.Cancel(j.ID)
		if err != nil {
			return nil, err
		}
		return out, nil
	case backend.MethodJobsList:
		if !HasCap(caps, "jobs.read") && !HasCap(caps, "jobs.schedule") {
			return nil, fmt.Errorf("capability jobs.read or jobs.schedule required")
		}
		if b.Jobs == nil {
			return nil, fmt.Errorf("jobs unavailable")
		}
		var p struct {
			Status string `json:"status"`
			Limit  int    `json:"limit"`
		}
		if len(params) > 0 && string(params) != "null" {
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
		}
		list, err := b.Jobs.List(strings.TrimSpace(p.Status), p.Limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"jobs": list}, nil
	case backend.MethodSystemStatus:
		if !HasCap(caps, "system.read") {
			return nil, fmt.Errorf("capability system.read required")
		}
		var roots []string
		if b.Files != nil {
			roots = b.Files.Roots()
		}
		resp := map[string]any{"disks": sysinfo.DiskForRoots(roots)}
		if load, err := sysinfo.Load(); err == nil {
			resp["load"] = load
		} else {
			resp["loadError"] = err.Error()
		}
		return resp, nil
	case backend.MethodEngineCapabilities:
		if !HasCap(caps, "engine.capabilities") {
			return nil, fmt.Errorf("capability engine.capabilities required")
		}
		if b.Engine == nil {
			return nil, fmt.Errorf("engine unavailable")
		}
		return b.Engine.Capabilities(), nil
	case backend.MethodTorrentsAdd:
		if !HasCap(caps, "torrents.add") {
			return nil, fmt.Errorf("capability torrents.add required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Magnets         []string `json:"magnets"`
			TorrentBase64   string   `json:"torrentBase64"`
			TorrentsBase64  []string `json:"torrentsBase64"`
			Start           *bool    `json:"start"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		start := true
		if p.Start != nil {
			start = *p.Start
		}
		added := 0
		if len(p.Magnets) > 0 {
			if err := b.Torrents.AddMagnets(ctx, p.Magnets, start); err != nil {
				return nil, err
			}
			added += len(p.Magnets)
		}
		payloads := append([]string{}, p.TorrentsBase64...)
		if strings.TrimSpace(p.TorrentBase64) != "" {
			payloads = append(payloads, p.TorrentBase64)
		}
		for _, b64 := range payloads {
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
			if err != nil {
				return nil, fmt.Errorf("invalid torrentBase64")
			}
			if err := b.Torrents.AddTorrentFile(ctx, data, start); err != nil {
				return nil, err
			}
			added++
		}
		if added == 0 {
			return nil, fmt.Errorf("magnets or torrentBase64 required")
		}
		return map[string]any{"ok": true, "count": added}, nil
	case backend.MethodIntakeAdd:
		if !HasCap(caps, "torrents.add") {
			return nil, fmt.Errorf("capability torrents.add required")
		}
		if b.Intake == nil {
			return nil, fmt.Errorf("intake unavailable")
		}
		var p struct {
			Magnets       []string `json:"magnets"`
			TorrentBase64 string   `json:"torrentBase64"`
			Filename      string   `json:"filename"`
			Start         *bool    `json:"start"`
			GatewayID     string   `json:"gatewayId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		start := true
		if p.Start != nil {
			start = *p.Start
		}
		var torrent []byte
		if strings.TrimSpace(p.TorrentBase64) != "" {
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(p.TorrentBase64))
			if err != nil {
				return nil, fmt.Errorf("invalid torrentBase64")
			}
			torrent = data
		}
		if len(p.Magnets) == 0 && len(torrent) == 0 {
			return nil, fmt.Errorf("magnets or torrentBase64 required")
		}
		gatewayID, err := b.Intake.Add(ctx, p.Magnets, torrent, p.Filename, start, p.GatewayID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "gatewayId": gatewayID}, nil
	case backend.MethodTorrentsList:
		if !HasCap(caps, "torrents.read") {
			return nil, fmt.Errorf("capability torrents.read required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			AllClients bool `json:"allClients"`
		}
		if len(params) > 0 && string(params) != "null" {
			_ = json.Unmarshal(params, &p)
		}
		rows, err := b.Torrents.ListExtTorrents(ctx, p.AllClients)
		if err != nil {
			return nil, err
		}
		return map[string]any{"torrents": rows}, nil
	case backend.MethodTorrentsAction:
		if !HasCap(caps, "torrents.actions") {
			return nil, fmt.Errorf("capability torrents.actions required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Action string   `json:"action"`
			Hashes []string `json:"hashes"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Torrents.ApplyExtAction(ctx, p.Action, p.Hashes); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "action": p.Action, "count": len(p.Hashes)}, nil
	case backend.MethodTorrentsSetDirectory:
		if !HasCap(caps, "torrents.actions") && !HasCap(caps, "torrents.write") {
			return nil, fmt.Errorf("capability torrents.actions required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash      string `json:"hash"`
			Directory string `json:"directory"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Torrents.SetDirectory(ctx, p.Hash, p.Directory); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case backend.MethodTorrentsLabel:
		if !HasCap(caps, "torrents.actions") && !HasCap(caps, "torrents.write") {
			return nil, fmt.Errorf("capability torrents.actions or torrents.write required")
		}
		var p struct {
			Hash     string `json:"hash"`
			Label    string `json:"label"`
			ClientID string `json:"clientId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		hash := strings.TrimSpace(p.Hash)
		if hash == "" {
			return nil, fmt.Errorf("hash required")
		}
		clientID := strings.TrimSpace(p.ClientID)
		if clientID == "" || clientID == "local-rtorrent" {
			if b.Torrents == nil {
				return nil, fmt.Errorf("torrent engine unavailable")
			}
			if err := b.Torrents.SetLabel(ctx, hash, p.Label); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "hash": hash, "label": p.Label, "clientId": "local-rtorrent"}, nil
		}
		if b.Gateways == nil {
			return nil, fmt.Errorf("gateways unavailable")
		}
		if err := b.Gateways.SetTorrentLabel(ctx, clientID, hash, p.Label); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "hash": hash, "label": p.Label, "clientId": clientID}, nil
	case backend.MethodTorrentsSetRates:
		if !HasCap(caps, "torrents.actions") && !HasCap(caps, "torrents.write") {
			return nil, fmt.Errorf("capability torrents.actions or torrents.write required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash     string   `json:"hash"`
			Hashes   []string `json:"hashes"`
			ClientID string   `json:"clientId"`
			UpRate   int64    `json:"upRate"`
			DownRate int64    `json:"downRate"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		ids := append([]string{}, p.Hashes...)
		if h := strings.TrimSpace(p.Hash); h != "" {
			ids = append(ids, h)
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("hash or hashes required")
		}
		applied := 0
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			hash, clientID := id, strings.TrimSpace(p.ClientID)
			if i := strings.LastIndex(id, "@"); i > 0 {
				hash = id[:i]
				if clientID == "" {
					clientID = id[i+1:]
				}
			}
			if err := b.Torrents.SetRates(ctx, hash, clientID, p.UpRate, p.DownRate); err != nil {
				return nil, fmt.Errorf("%s: %w", strings.ToUpper(hash), err)
			}
			applied++
		}
		return map[string]any{"ok": true, "applied": applied, "upRate": p.UpRate, "downRate": p.DownRate}, nil
	case backend.MethodFilesRoots:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		return map[string]any{"roots": b.Files.Roots()}, nil
	case backend.MethodFilesList:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(params, &p)
		entries, cwd, err := b.Files.List(p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"cwd": cwd, "entries": entries}, nil
	case backend.MethodFilesMkdir:
		if !HasCap(caps, "files.write") {
			return nil, fmt.Errorf("capability files.write required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Files.Mkdir(p.Path); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "path": p.Path}, nil
	case backend.MethodFilesRename, backend.MethodFilesMove:
		if !HasCap(caps, "files.write") {
			return nil, fmt.Errorf("capability files.write required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		var err error
		if method == backend.MethodFilesMove {
			err = b.Files.Move(p.From, p.To)
		} else {
			err = b.Files.Rename(p.From, p.To)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "from": p.From, "to": p.To}, nil
	case backend.MethodFilesRemove:
		if !HasCap(caps, "files.write") {
			return nil, fmt.Errorf("capability files.write required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path      string `json:"path"`
			Recursive bool   `json:"recursive"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Files.Remove(p.Path, p.Recursive); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "path": p.Path}, nil
	case backend.MethodFilesCopy:
		if !HasCap(caps, "files.write") {
			return nil, fmt.Errorf("capability files.write required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Files.Copy(p.From, p.To); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "from": p.From, "to": p.To}, nil
	case backend.MethodFilesReadText:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		text, err := b.Files.ReadText(p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": p.Path, "text": text}, nil
	case backend.MethodFilesChecksum:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		algo, hex, size, err := b.Files.Checksum(p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": p.Path, "algo": algo, "hash": hex, "size": size}, nil
	case backend.MethodFilesReadBase64:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		dataURL, size, err := b.Files.ReadBase64(p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": p.Path, "dataUrl": dataURL, "size": size}, nil
	case backend.MethodFilesStat:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		st, err := b.Files.Stat(p.Path)
		if err != nil {
			return nil, err
		}
		return st, nil
	case backend.MethodFilesFingerprint:
		if !HasCap(caps, "files.read") {
			return nil, fmt.Errorf("capability files.read required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			Path   string `json:"path"`
			NBytes int    `json:"nBytes"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		size, digest, err := b.Files.Fingerprint(p.Path, p.NBytes)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": p.Path, "size": size, "sha256": digest}, nil
	case backend.MethodFilesHardlink:
		if !HasCap(caps, "files.write") {
			return nil, fmt.Errorf("capability files.write required")
		}
		if b.Files == nil {
			return nil, fmt.Errorf("filesystem unavailable")
		}
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Files.Hardlink(p.From, p.To); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "from": p.From, "to": p.To}, nil
	case backend.MethodFilesUnpack:
		if !HasCap(caps, "files.unpack") {
			return nil, fmt.Errorf("capability files.unpack required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Archive string `json:"archive"`
			Dest    string `json:"dest"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		n, err := b.Torrents.UnpackArchive(ctx, p.Archive, p.Dest)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "extracted": n, "archive": p.Archive, "dest": p.Dest}, nil
	case backend.MethodFilesMediainfo:
		if !HasCap(caps, "files.mediainfo") {
			return nil, fmt.Errorf("capability files.mediainfo required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash  string `json:"hash"`
			Index int    `json:"index"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		text, err := b.Torrents.MediaInfo(ctx, p.Hash, p.Index, p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "text": text}, nil
	case backend.MethodFilesScreenshots:
		if !HasCap(caps, "files.screenshots") {
			return nil, fmt.Errorf("capability files.screenshots required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash     string `json:"hash"`
			Index    int    `json:"index"`
			Path     string `json:"path"`
			Count    int    `json:"count"`
			Offset   int    `json:"offset"`
			Interval int    `json:"interval"`
			Width    int    `json:"width"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		frames, err := b.Torrents.Screenshots(ctx, p.Hash, p.Index, p.Path, p.Count, p.Offset, p.Interval, p.Width)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "frames": frames, "count": len(frames)}, nil
	case backend.MethodFilesSpectrogram:
		if !HasCap(caps, "files.spectrogram") {
			return nil, fmt.Errorf("capability files.spectrogram required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash  string `json:"hash"`
			Index int    `json:"index"`
			Path  string `json:"path"`
			Title string `json:"title"`
			Width int    `json:"width"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		img, err := b.Torrents.Spectrogram(ctx, p.Hash, p.Index, p.Path, p.Title, p.Width)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "image": img}, nil
	case backend.MethodTrackersList:
		if !HasCap(caps, "trackers.read") {
			return nil, fmt.Errorf("capability trackers.read required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		rows, err := b.Torrents.ListTrackers(ctx, p.Hash)
		if err != nil {
			return nil, err
		}
		return map[string]any{"hash": strings.ToUpper(strings.TrimSpace(p.Hash)), "trackers": rows}, nil
	case backend.MethodPeersList:
		if !HasCap(caps, "peers.read") {
			return nil, fmt.Errorf("capability peers.read required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		rows, err := b.Torrents.ListPeers(ctx, p.Hash)
		if err != nil {
			return nil, err
		}
		return map[string]any{"hash": strings.ToUpper(strings.TrimSpace(p.Hash)), "peers": rows}, nil
	case backend.MethodPeersDisconnect:
		if !HasCap(caps, "peers.actions") {
			return nil, fmt.Errorf("capability peers.actions required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash    string `json:"hash"`
			Address string `json:"address"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Torrents.DisconnectPeer(ctx, p.Hash, p.Address); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case backend.MethodTrackersAdd:
		if !HasCap(caps, "trackers.write") {
			return nil, fmt.Errorf("capability trackers.write required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash   string   `json:"hash"`
			URL    string   `json:"url"`
			URLs   []string `json:"urls"`
			Hashes []string `json:"hashes"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		urls := append([]string{}, p.URLs...)
		if strings.TrimSpace(p.URL) != "" {
			urls = append(urls, p.URL)
		}
		hashes := append([]string{}, p.Hashes...)
		if strings.TrimSpace(p.Hash) != "" {
			hashes = append(hashes, p.Hash)
		}
		if len(hashes) == 0 {
			return nil, fmt.Errorf("hash or hashes required")
		}
		if len(urls) == 0 {
			return nil, fmt.Errorf("url or urls required")
		}
		added := 0
		for _, hash := range hashes {
			hash = strings.ToUpper(strings.TrimSpace(hash))
			if hash == "" {
				continue
			}
			for _, u := range urls {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				if err := b.Torrents.AddTracker(ctx, hash, u); err != nil {
					return nil, err
				}
				added++
			}
		}
		return map[string]any{"ok": true, "added": added}, nil
	case backend.MethodTorrentsMetainfo:
		if !HasCap(caps, "torrents.read") {
			return nil, fmt.Errorf("capability torrents.read required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		hash := strings.ToUpper(strings.TrimSpace(p.Hash))
		if hash == "" {
			return nil, fmt.Errorf("hash required")
		}
		comment, announce, err := b.Torrents.TorrentMetainfo(ctx, hash)
		if err != nil {
			return nil, err
		}
		return map[string]any{"hash": hash, "comment": comment, "announce": announce}, nil
	case backend.MethodTorrentsSource:
		if !HasCap(caps, "torrents.read") {
			return nil, fmt.Errorf("capability torrents.read required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		hash := strings.ToUpper(strings.TrimSpace(p.Hash))
		if hash == "" {
			return nil, fmt.Errorf("hash required")
		}
		filename, data, err := b.Torrents.ResolveSource(ctx, hash)
		if err != nil {
			return nil, err
		}
		if len(data) > 8<<20 {
			return nil, fmt.Errorf("torrent source too large")
		}
		return map[string]any{
			"hash":          hash,
			"name":          filename,
			"torrentBase64": base64.StdEncoding.EncodeToString(data),
			"bytes":         len(data),
		}, nil
	case backend.MethodGatewaysList:
		if !HasCap(caps, "gateways.read") {
			return nil, fmt.Errorf("capability gateways.read required")
		}
		if b.Gateways == nil {
			return nil, fmt.Errorf("gateways unavailable")
		}
		list, err := b.Gateways.ListGateways(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"clients": list}, nil
	case backend.MethodGatewaysAddTorrent:
		if !HasCap(caps, "gateways.write") {
			return nil, fmt.Errorf("capability gateways.write required")
		}
		if b.Gateways == nil {
			return nil, fmt.Errorf("gateways unavailable")
		}
		var p struct {
			GatewayID     string `json:"gatewayId"`
			Filename      string `json:"filename"`
			TorrentBase64 string `json:"torrentBase64"`
			SavePath      string `json:"savePath"`
			SkipIfPresent bool   `json:"skipIfPresent"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		id := strings.TrimSpace(p.GatewayID)
		if id == "" {
			return nil, fmt.Errorf("gatewayId required")
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(p.TorrentBase64))
		if err != nil || len(raw) == 0 {
			return nil, fmt.Errorf("torrentBase64 required")
		}
		if len(raw) > 8<<20 {
			return nil, fmt.Errorf("torrent too large")
		}
		savePath := strings.TrimSpace(p.SavePath)
		if savePath == "" {
			return nil, fmt.Errorf("savePath required")
		}
		ok, skipped, message, err := b.Gateways.AddTorrent(ctx, id, p.Filename, raw, savePath, p.SkipIfPresent)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": ok, "skipped": skipped, "message": message}, nil
	case backend.MethodProcessExec:
		if !HasCap(caps, "process.exec") {
			return nil, fmt.Errorf("capability process.exec required")
		}
		var p struct {
			Path       string   `json:"path"`
			Args       []string `json:"args"`
			TimeoutSec int      `json:"timeoutSec"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return runAllowlistedExec(ctx, p.Path, p.Args, p.TimeoutSec)
	case backend.MethodHTTPFetch:
		if !HasCap(caps, "http.fetch") {
			return nil, fmt.Errorf("capability http.fetch required")
		}
		var p struct {
			URL         string            `json:"url"`
			Method      string            `json:"method"`
			Body        string            `json:"body"`
			BodyBase64  string            `json:"bodyBase64"`
			ContentType string            `json:"contentType"`
			Headers     map[string]string `json:"headers"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		u := strings.TrimSpace(p.URL)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return nil, fmt.Errorf("only http(s) URLs allowed")
		}
		method := strings.ToUpper(strings.TrimSpace(p.Method))
		if method == "" {
			method = http.MethodGet
		}
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodHead:
		default:
			return nil, fmt.Errorf("http method %q not allowed", method)
		}
		var bodyReader io.Reader
		if strings.TrimSpace(p.BodyBase64) != "" {
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(p.BodyBase64))
			if err != nil {
				return nil, fmt.Errorf("invalid bodyBase64")
			}
			if len(raw) > 1<<20 {
				return nil, fmt.Errorf("body too large")
			}
			bodyReader = bytes.NewReader(raw)
		} else if p.Body != "" {
			if len(p.Body) > 1<<20 {
				return nil, fmt.Errorf("body too large")
			}
			bodyReader = strings.NewReader(p.Body)
		}
		client := b.HTTP
		if client == nil {
			client = &http.Client{Timeout: 30 * time.Second}
		}
		req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Fleet-Extension/1.0")
		if ct := strings.TrimSpace(p.ContentType); ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		for k, v := range p.Headers {
			k = strings.TrimSpace(k)
			if k == "" || strings.EqualFold(k, "Cookie") || strings.EqualFold(k, "Host") {
				continue
			}
			req.Header.Set(k, v)
		}
		if b.Cookies != nil {
			if cookie, err := b.Cookies.HeaderForURL(u); err == nil && cookie != "" {
				req.Header.Set("Cookie", cookie)
			}
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"status":     res.StatusCode,
			"body":       string(body),
			"bodyBase64": base64.StdEncoding.EncodeToString(body),
		}, nil
	case backend.MethodCookiesList:
		if !HasCap(caps, "cookies.read") {
			return nil, fmt.Errorf("capability cookies.read required")
		}
		if b.Cookies == nil {
			return nil, fmt.Errorf("cookie store unavailable")
		}
		entries, err := b.Cookies.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"hosts": entries}, nil
	case backend.MethodCookiesSet:
		if !HasCap(caps, "cookies.write") {
			return nil, fmt.Errorf("capability cookies.write required")
		}
		if b.Cookies == nil {
			return nil, fmt.Errorf("cookie store unavailable")
		}
		var p struct {
			Hosts []HostCookieEntry `json:"hosts"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Cookies.Set(p.Hosts); err != nil {
			return nil, err
		}
		entries, err := b.Cookies.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "hosts": entries}, nil
	case backend.MethodLoginMgrList:
		if !HasCap(caps, "loginmgr.read") {
			return nil, fmt.Errorf("capability loginmgr.read required")
		}
		if b.LoginMgr == nil {
			return nil, fmt.Errorf("loginmgr store unavailable")
		}
		accounts, err := b.LoginMgr.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"accounts": accounts}, nil
	case backend.MethodLoginMgrSet:
		if !HasCap(caps, "loginmgr.write") {
			return nil, fmt.Errorf("capability loginmgr.write required")
		}
		if b.LoginMgr == nil {
			return nil, fmt.Errorf("loginmgr store unavailable")
		}
		var p struct {
			Accounts []LoginAccount `json:"accounts"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.LoginMgr.Set(p.Accounts); err != nil {
			return nil, err
		}
		accounts, err := b.LoginMgr.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "accounts": accounts}, nil
	case backend.MethodLoginMgrLogin:
		if !HasCap(caps, "loginmgr.write") {
			return nil, fmt.Errorf("capability loginmgr.write required")
		}
		if b.LoginMgr == nil {
			return nil, fmt.Errorf("loginmgr store unavailable")
		}
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		host, count, err := b.LoginMgr.Login(ctx, p.Name)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "host": host, "cookieCount": count}, nil
	case backend.MethodURLRewriteList:
		if !HasCap(caps, "urlrewrite.read") {
			return nil, fmt.Errorf("capability urlrewrite.read required")
		}
		if b.URLRewrite == nil {
			return nil, fmt.Errorf("urlrewrite store unavailable")
		}
		rules, err := b.URLRewrite.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"rules": rules}, nil
	case backend.MethodURLRewriteSet:
		if !HasCap(caps, "urlrewrite.write") {
			return nil, fmt.Errorf("capability urlrewrite.write required")
		}
		if b.URLRewrite == nil {
			return nil, fmt.Errorf("urlrewrite store unavailable")
		}
		var p struct {
			Rules []URLRewriteRule `json:"rules"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.URLRewrite.Set(p.Rules); err != nil {
			return nil, err
		}
		rules, err := b.URLRewrite.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "rules": rules}, nil
	case backend.MethodURLRewriteApply:
		if !HasCap(caps, "urlrewrite.read") {
			return nil, fmt.Errorf("capability urlrewrite.read required")
		}
		if b.URLRewrite == nil {
			return nil, fmt.Errorf("urlrewrite store unavailable")
		}
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		out, changed, matched, err := b.URLRewrite.Apply(p.URL)
		if err != nil {
			return nil, err
		}
		return map[string]any{"url": out, "changed": changed, "matchedRule": matched}, nil
	case backend.MethodThrottleGet:
		if !HasCap(caps, "settings.read") {
			return nil, fmt.Errorf("capability settings.read required")
		}
		if b.Throttle == nil {
			return nil, fmt.Errorf("throttle control unavailable")
		}
		down, up, err := b.Throttle.GetGlobalRates(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"downRate": down, "upRate": up}, nil
	case backend.MethodThrottleSet:
		if !HasCap(caps, "settings.write") {
			return nil, fmt.Errorf("capability settings.write required")
		}
		if b.Throttle == nil {
			return nil, fmt.Errorf("throttle control unavailable")
		}
		var p struct {
			DownRate int64 `json:"downRate"`
			UpRate   int64 `json:"upRate"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Throttle.SetGlobalRates(ctx, p.DownRate, p.UpRate); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "downRate": p.DownRate, "upRate": p.UpRate}, nil
	case backend.MethodThrottleDefine:
		if !HasCap(caps, "settings.write") {
			return nil, fmt.Errorf("capability settings.write required")
		}
		if b.Throttle == nil {
			return nil, fmt.Errorf("throttle control unavailable")
		}
		var p struct {
			Name     string `json:"name"`
			UpRate   int64  `json:"upRate"`
			DownRate int64  `json:"downRate"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := b.Throttle.EnsureNamed(ctx, p.Name, p.UpRate, p.DownRate); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "name": strings.TrimSpace(p.Name), "upRate": p.UpRate, "downRate": p.DownRate}, nil
	case backend.MethodThrottleAssign:
		if !HasCap(caps, "torrents.actions") && !HasCap(caps, "torrents.write") {
			return nil, fmt.Errorf("capability torrents.actions required")
		}
		if b.Torrents == nil {
			return nil, fmt.Errorf("torrent engine unavailable")
		}
		var p struct {
			Hashes []string `json:"hashes"`
			Name   string   `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if len(p.Hashes) == 0 {
			return nil, fmt.Errorf("hashes required")
		}
		if err := b.Torrents.AssignThrottles(ctx, p.Hashes, p.Name); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "count": len(p.Hashes), "name": strings.TrimSpace(p.Name)}, nil
	default:
		return nil, fmt.Errorf("method not allowed: %s", method)
	}
}
