package rtorrent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Action is a torrent control operation exposed by Fleet.
type Action string

const (
	ActionStart      Action = "start"
	ActionStop       Action = "stop"
	ActionPause      Action = "pause"
	ActionResume     Action = "resume"
	ActionHash       Action = "hash"
	ActionErase      Action = "erase"
	ActionEraseData  Action = "erase_data"
	ActionReannounce Action = "reannounce"
)

// ActionResult reports per-hash outcome.
type ActionResult struct {
	Hash     string `json:"hash"`
	ClientID string `json:"clientId,omitempty"`
	OK       bool   `json:"ok"`
	Message  string `json:"message,omitempty"`
}

// ApplyAction runs a control operation for each hash sequentially.
// downloadRoots is required for erase_data; paths must resolve under a root.
func (c *Client) ApplyAction(ctx context.Context, action Action, hashes []string, downloadRoots []string) []ActionResult {
	out := make([]ActionResult, 0, len(hashes))
	for _, hash := range hashes {
		hash = strings.ToUpper(strings.TrimSpace(hash))
		if hash == "" {
			out = append(out, ActionResult{OK: false, Message: "empty hash"})
			continue
		}
		err := c.applyOne(ctx, action, hash, downloadRoots)
		if err != nil {
			out = append(out, ActionResult{Hash: hash, OK: false, Message: err.Error()})
			continue
		}
		out = append(out, ActionResult{Hash: hash, OK: true})
	}
	return out
}

func (c *Client) applyOne(ctx context.Context, action Action, hash string, downloadRoots []string) error {
	switch action {
	case ActionStart:
		if _, err := c.Call(ctx, "d.open", hash); err != nil {
			// Some builds already open; continue to start.
			_ = err
		}
		_, err := c.Call(ctx, "d.start", hash)
		return err
	case ActionStop:
		_, err := c.Call(ctx, "d.stop", hash)
		return err
	case ActionPause:
		if _, err := c.Call(ctx, "d.pause", hash); err != nil {
			_, err = c.Call(ctx, "d.stop", hash)
			return err
		}
		return nil
	case ActionResume:
		if _, err := c.Call(ctx, "d.resume", hash); err != nil {
			if _, err := c.Call(ctx, "d.open", hash); err != nil {
				_ = err
			}
			_, err = c.Call(ctx, "d.start", hash)
			return err
		}
		return nil
	case ActionHash:
		_, err := c.Call(ctx, "d.check_hash", hash)
		return err
	case ActionReannounce:
		return c.reannounce(ctx, hash)
	case ActionErase:
		return c.erase(ctx, hash, false, downloadRoots)
	case ActionEraseData:
		return c.erase(ctx, hash, true, downloadRoots)
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}

// reannounce forces tracker announce/scrape for a torrent (AddTorrentCheck-like).
func (c *Client) reannounce(ctx context.Context, hash string) error {
	// Prefer per-tracker force_reannounce when available.
	if _, err := c.Call(ctx, "t.multicall", hash, "", "t.force_reannounce="); err == nil {
		return nil
	}
	if _, err := c.Call(ctx, "d.tracker.send_scrape", hash); err == nil {
		return nil
	}
	// Last resort: stop/start cycle nudges announce on some builds.
	_, _ = c.Call(ctx, "d.stop", hash)
	if _, err := c.Call(ctx, "d.open", hash); err != nil {
		_ = err
	}
	_, err := c.Call(ctx, "d.start", hash)
	return err
}

func (c *Client) erase(ctx context.Context, hash string, withData bool, downloadRoots []string) error {
	var basePath string
	if withData {
		if len(downloadRoots) == 0 {
			return fmt.Errorf("erase_data refused: configure rtorrent.download_roots")
		}
		p, err := c.resolveDataPath(ctx, hash)
		if err != nil {
			return err
		}
		basePath = p
		if err := assertUnderRoots(basePath, downloadRoots); err != nil {
			return err
		}
	}

	if _, err := c.Call(ctx, "d.close", hash); err != nil {
		_ = err // may already be closed
	}
	if withData && basePath != "" {
		if err := os.RemoveAll(basePath); err != nil {
			return fmt.Errorf("delete data: %w", err)
		}
	}
	_, err := c.Call(ctx, "d.erase", hash)
	return err
}

// resolveDataPath picks the on-disk payload for erase_data.
// Closed/never-started torrents often report empty d.base_path; fall back to
// directory (+ name for single-file) without ever returning a bare download root.
func (c *Client) resolveDataPath(ctx context.Context, hash string) (string, error) {
	if v, err := c.Call(ctx, "d.base_path", hash); err == nil {
		if p := strings.TrimSpace(v.AsString()); p != "" {
			return p, nil
		}
	}
	dir := ""
	if v, err := c.Call(ctx, "d.directory", hash); err == nil {
		dir = strings.TrimSpace(v.AsString())
	}
	if dir == "" {
		return "", fmt.Errorf("resolve data path: empty base_path and directory")
	}
	name := ""
	if v, err := c.Call(ctx, "d.name", hash); err == nil {
		name = strings.TrimSpace(v.AsString())
	}
	// Multi-file layout: directory already points at the torrent folder.
	if name != "" && filepath.Base(filepath.Clean(dir)) == name {
		return dir, nil
	}
	if name != "" {
		return filepath.Join(dir, name), nil
	}
	return "", fmt.Errorf("resolve data path: empty base_path and name")
}

func assertUnderRoots(target string, roots []string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("empty torrent data path")
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	resolvedTarget, err := filepath.EvalSymlinks(absTarget)
	if err != nil {
		// Path may not exist yet; still validate lexical containment on Abs path.
		resolvedTarget = absTarget
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		resolvedRoot, err := filepath.EvalSymlinks(absRoot)
		if err != nil {
			resolvedRoot = absRoot
		}
		rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
		if err != nil {
			continue
		}
		// Refuse deleting a download root itself (would wipe the library).
		if rel == "." {
			return fmt.Errorf("path %q equals download root; refusing erase_data", absTarget)
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil
		}
	}
	return fmt.Errorf("path %q is outside configured download_roots", absTarget)
}
