package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/krate-apps/fleet/internal/metainfo"
	"github.com/krate-apps/fleet/internal/rtorrent"
	"github.com/krate-apps/fleet/internal/unpack"
)

// ExtTorrentRow is the slim torrent view exposed to extension workers.
type ExtTorrentRow struct {
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

func (e *Engine) withClient(fn func(*rtorrent.Client) error) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("rtorrent not configured")
	}
	return fn(e.client)
}

// ApplyAction runs a torrent action under the write mutex (RPC-008).
// Each hash may be "INFOHASH" or "INFOHASH@clientId" (PD-GW-10).
func (e *Engine) ApplyAction(ctx context.Context, action rtorrent.Action, hashes []string, downloadRoots []string) ([]rtorrent.ActionResult, error) {
	targets := make([]ActionTarget, 0, len(hashes))
	for _, h := range hashes {
		hash, cid := splitTorrentID(h)
		targets = append(targets, ActionTarget{Hash: hash, ClientID: cid})
	}
	return e.ApplyActionTargets(ctx, action, targets, downloadRoots)
}

// AddMagnets loads magnet URIs.
func (e *Engine) AddMagnets(ctx context.Context, magnets []string, start bool) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.AddMagnet(ctx, magnets, start); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// AddTorrentFile loads a .torrent payload.
func (e *Engine) AddTorrentFile(ctx context.Context, data []byte, start bool) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.AddTorrentFile(ctx, data, start); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// ListFiles proxies f.multicall for a torrent.
func (e *Engine) ListFiles(ctx context.Context, hash string) ([]rtorrent.FileEntry, error) {
	var out []rtorrent.FileEntry
	err := e.withClient(func(c *rtorrent.Client) error {
		var err error
		out, err = c.ListFiles(ctx, hash)
		return err
	})
	return out, err
}

// ListTrackers proxies t.multicall for a torrent.
func (e *Engine) ListTrackers(ctx context.Context, hash string) ([]rtorrent.TrackerEntry, error) {
	var out []rtorrent.TrackerEntry
	err := e.withClient(func(c *rtorrent.Client) error {
		var err error
		out, err = c.ListTrackers(ctx, hash)
		return err
	})
	return out, err
}

// ListPeers proxies p.multicall for a torrent.
func (e *Engine) ListPeers(ctx context.Context, hash string) ([]rtorrent.PeerEntry, error) {
	var out []rtorrent.PeerEntry
	err := e.withClient(func(c *rtorrent.Client) error {
		var err error
		out, err = c.ListPeers(ctx, hash)
		return err
	})
	return out, err
}

// DisconnectPeer proxies p.disconnect for a matched peer address.
func (e *Engine) DisconnectPeer(ctx context.Context, hash, address string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		return c.DisconnectPeer(ctx, hash, address)
	})
}

// SetLabel updates d.custom1 for a torrent.
func (e *Engine) SetLabel(ctx context.Context, hash, label string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.SetLabel(ctx, hash, label); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// SetDirectory updates d.directory for a torrent (after data move).
func (e *Engine) SetDirectory(ctx context.Context, hash, directory string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.SetDirectory(ctx, hash, directory); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// SetName updates d.name for a torrent (display rename).
func (e *Engine) SetName(ctx context.Context, hash, name string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.SetName(ctx, hash, name); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// SetTorrentPriority updates d.priority.
func (e *Engine) SetTorrentPriority(ctx context.Context, hash string, priority int64) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.SetTorrentPriority(ctx, hash, priority); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// SetFilePriorities updates f.priority for selected indexes.
func (e *Engine) SetFilePriorities(ctx context.Context, hash string, indexes []int64, priority int64) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		if err := c.SetFilePriorities(ctx, hash, indexes, priority); err != nil {
			return err
		}
		e.Kick()
		return nil
	})
}

// AddTracker inserts a tracker URL.
func (e *Engine) AddTracker(ctx context.Context, hash, trackerURL string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		return c.AddTracker(ctx, hash, trackerURL)
	})
}

// SetTrackerEnabled toggles a tracker.
func (e *Engine) SetTrackerEnabled(ctx context.Context, trackerID string, enabled bool) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		return c.SetTrackerEnabled(ctx, trackerID, enabled)
	})
}

// RemoveTracker deletes a tracker.
func (e *Engine) RemoveTracker(ctx context.Context, trackerID string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		return c.RemoveTracker(ctx, trackerID)
	})
}

// ListExtTorrents returns a slim torrent list for extension workers (SDK torrents.read).
// When allClients is false (default), only the primary rTorrent rows are returned.
func (e *Engine) ListExtTorrents(ctx context.Context, allClients bool) ([]ExtTorrentRow, error) {
	_ = ctx
	_, list, st := e.Snapshot()
	if !st.Connected && len(list) == 0 {
		return nil, fmt.Errorf("no torrent sources connected")
	}
	out := make([]ExtTorrentRow, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, t := range list {
		if !allClients && t.ClientID != "" && t.ClientID != localRTorrentClientID {
			continue
		}
		key := t.Hash
		if allClients {
			key = t.ClientID + ":" + t.Hash
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clientID := t.ClientID
		if clientID == "" {
			clientID = localRTorrentClientID
		}
		out = append(out, ExtTorrentRow{
			Hash:         t.Hash,
			Name:         t.Name,
			Ratio:        t.Ratio,
			Label:        t.Label,
			State:        t.State,
			SizeBytes:    t.SizeBytes,
			DownTotal:    t.DownTotal,
			UpTotal:      t.UpTotal,
			DownRate:     t.DownRate,
			UpRate:       t.UpRate,
			Complete:     t.Complete,
			FinishedAt:   t.FinishedAt,
			Directory:    t.Directory,
			ThrottleName: t.ThrottleName,
			Private:      t.Private,
			ClientID:     clientID,
		})
	}
	return out, nil
}

// EnsureNamedThrottle creates/updates a named rTorrent throttle profile.
func (e *Engine) EnsureNamedThrottle(ctx context.Context, name string, upRate, downRate int64) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		return c.EnsureNamedThrottle(ctx, name, upRate, downRate)
	})
}

// AssignTorrentThrottles sets d.throttle_name for each hash (empty name clears).
func (e *Engine) AssignTorrentThrottles(ctx context.Context, hashes []string, name string) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		for _, h := range hashes {
			if err := c.SetTorrentThrottleName(ctx, h, name); err != nil {
				return err
			}
		}
		e.Kick()
		return nil
	})
}

// UnpackOptions controls archive extraction helpers.
type UnpackOptions struct {
	DownloadRoots []string
	UnrarBin      string
}

// UnpackArchive extracts a zip/rar under download_roots (SDK files.unpack).
func (e *Engine) UnpackArchive(ctx context.Context, archivePath, destDir string, opts UnpackOptions) (int, error) {
	_ = ctx
	archivePath = strings.TrimSpace(archivePath)
	destDir = strings.TrimSpace(destDir)
	if archivePath == "" {
		return 0, fmt.Errorf("archive required")
	}
	if destDir == "" {
		destDir = unpack.DefaultDest(archivePath)
	}
	switch strings.ToLower(filepath.Ext(archivePath)) {
	case ".zip":
		return unpack.Zip(archivePath, destDir, opts.DownloadRoots)
	case ".rar":
		return unpack.Rar(archivePath, destDir, opts.DownloadRoots, opts.UnrarBin)
	default:
		return 0, fmt.Errorf("unsupported archive type %q (want .zip or .rar)", filepath.Ext(archivePath))
	}
}

// ApplyExtAction runs a allowlisted control action for extension workers.
func (e *Engine) ApplyExtAction(ctx context.Context, action string, hashes []string) error {
	a := rtorrent.Action(strings.ToLower(strings.TrimSpace(action)))
	switch a {
	case rtorrent.ActionStart, rtorrent.ActionStop, rtorrent.ActionPause, rtorrent.ActionResume,
		rtorrent.ActionHash, rtorrent.ActionErase, rtorrent.ActionReannounce:
	default:
		return fmt.Errorf("action %q not allowed for extensions", action)
	}
	results, err := e.ApplyAction(ctx, a, hashes, nil)
	if err != nil {
		return err
	}
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("%s %s: %s", a, r.Hash, r.Message)
		}
	}
	return nil
}

// ResolveSourceTorrent locates a downloadable session/tied .torrent for hash.
func (e *Engine) ResolveSourceTorrent(ctx context.Context, hash string, downloadRoots []string) (path string, filename string, err error) {
	err = e.withClient(func(c *rtorrent.Client) error {
		var e2 error
		path, filename, e2 = c.ResolveSourceTorrent(ctx, hash, downloadRoots)
		return e2
	})
	return path, filename, err
}

// TorrentMetainfo returns comment/announce from the session or tied .torrent.
func (e *Engine) TorrentMetainfo(ctx context.Context, hash string, downloadRoots []string) (comment, announce string, err error) {
	path, _, err := e.ResolveSourceTorrent(ctx, hash, downloadRoots)
	if err != nil {
		return "", "", err
	}
	sum, err := metainfo.ParseTorrentFile(path)
	if err != nil {
		return "", "", err
	}
	return sum.Comment, sum.Announce, nil
}

// ResolveDataFile locates a downloadable torrent payload file under download_roots.
func (e *Engine) ResolveDataFile(ctx context.Context, hash string, index int, downloadRoots []string) (path string, filename string, err error) {
	err = e.withClient(func(c *rtorrent.Client) error {
		var e2 error
		path, filename, e2 = c.ResolveDataFile(ctx, hash, index, downloadRoots)
		return e2
	})
	return path, filename, err
}

// GetChunks returns the piece bitfield map for a torrent.
func (e *Engine) GetChunks(ctx context.Context, hash string) (rtorrent.ChunkMap, error) {
	var out rtorrent.ChunkMap
	err := e.withClient(func(c *rtorrent.Client) error {
		var e2 error
		out, e2 = c.GetChunks(ctx, hash)
		return e2
	})
	return out, err
}

// GetGlobalRates reads rTorrent global throttle limits.
func (e *Engine) GetGlobalRates(ctx context.Context) (rtorrent.GlobalRates, error) {
	var out rtorrent.GlobalRates
	err := e.withClient(func(c *rtorrent.Client) error {
		var err error
		out, err = c.GetGlobalRates(ctx)
		return err
	})
	return out, err
}

// SetGlobalRates sets rTorrent global throttle limits (bytes/sec; 0 unlimited).
func (e *Engine) SetGlobalRates(ctx context.Context, downRate, upRate int64) error {
	return e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		return c.SetGlobalRates(ctx, downRate, upRate)
	})
}
