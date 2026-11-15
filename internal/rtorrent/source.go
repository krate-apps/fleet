package rtorrent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxSourceTorrentBytes = 32 << 20 // 32 MiB

// ResolveSourceTorrent finds the session or tied .torrent for hash (ruTorrent "source" plugin).
// The file must be a regular file under the rTorrent session directory and/or download_roots.
func (c *Client) ResolveSourceTorrent(ctx context.Context, hash string, downloadRoots []string) (path string, filename string, err error) {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if len(hash) != 40 {
		return "", "", fmt.Errorf("invalid torrent hash")
	}

	sessionDir := ""
	if v, err := c.Call(ctx, "session.path"); err == nil {
		sessionDir = strings.TrimSpace(v.AsString())
	}
	if sessionDir == "" {
		if v, err := c.Call(ctx, "get_session"); err == nil {
			sessionDir = strings.TrimSpace(v.AsString())
		}
	}

	tied := ""
	if v, err := c.Call(ctx, "d.tied_to_file", hash); err == nil {
		tied = strings.TrimSpace(v.AsString())
	}
	if tied == "" {
		if v, err := c.Call(ctx, "d.get_tied_to_file", hash); err == nil {
			tied = strings.TrimSpace(v.AsString())
		}
	}

	name := hash
	if v, err := c.Call(ctx, "d.name", hash); err == nil {
		if n := strings.TrimSpace(v.AsString()); n != "" {
			name = n
		}
	}
	downloadName := sanitizeTorrentFilename(name) + ".torrent"

	candidates := sourceCandidates(hash, sessionDir, tied)
	roots := append([]string{}, downloadRoots...)
	if sessionDir != "" {
		roots = append(roots, sessionDir)
	}
	if len(roots) == 0 {
		return "", "", fmt.Errorf("source download refused: configure rtorrent.download_roots or ensure session.path is available")
	}

	var lastErr error
	for _, cand := range candidates {
		if err := validateSourceFile(cand, roots); err != nil {
			lastErr = err
			continue
		}
		return cand, downloadName, nil
	}
	if lastErr != nil {
		return "", "", lastErr
	}
	return "", "", fmt.Errorf("source .torrent not found for %s", hash)
}

func sourceCandidates(hash, sessionDir, tied string) []string {
	out := make([]string, 0, 3)
	seen := map[string]struct{}{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if sessionDir != "" {
		add(filepath.Join(sessionDir, hash+".torrent"))
	}
	add(tied)
	return out
}

func validateSourceFile(path string, roots []string) error {
	if err := assertUnderRoots(path, roots); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	st, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source path %q is a symlink; refused", abs)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("source path %q is not a regular file", abs)
	}
	if st.Size() <= 0 {
		return fmt.Errorf("source path %q is empty", abs)
	}
	if st.Size() > maxSourceTorrentBytes {
		return fmt.Errorf("source path %q exceeds %d byte limit", abs, maxSourceTorrentBytes)
	}
	return nil
}

func sanitizeTorrentFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "torrent"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "torrent"
	}
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}
