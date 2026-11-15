package rtorrent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxDataFileBytes = 4 << 30 // 4 GiB

// ResolveDataFile locates an on-disk torrent payload file (ruTorrent "data" plugin).
// The path must be a regular file under download_roots (no symlink, size capped).
func (c *Client) ResolveDataFile(ctx context.Context, hash string, index int, downloadRoots []string) (path string, filename string, err error) {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if len(hash) != 40 {
		return "", "", fmt.Errorf("invalid torrent hash")
	}
	if index < 0 {
		return "", "", fmt.Errorf("invalid file index")
	}
	if len(downloadRoots) == 0 {
		return "", "", fmt.Errorf("data download refused: configure rtorrent.download_roots")
	}

	frozen := ""
	if v, err := c.Call(ctx, "f.frozen_path", hash, index); err == nil {
		frozen = strings.TrimSpace(v.AsString())
	}
	if frozen == "" {
		if v, err := c.Call(ctx, "f.get_frozen_path", hash, index); err == nil {
			frozen = strings.TrimSpace(v.AsString())
		}
	}
	if frozen == "" {
		// Fallback: directory + relative path from f.multicall.
		files, err := c.ListFiles(ctx, hash)
		if err != nil {
			return "", "", err
		}
		if index >= len(files) {
			return "", "", fmt.Errorf("file index %d out of range", index)
		}
		dir := ""
		if v, err := c.Call(ctx, "d.directory", hash); err == nil {
			dir = strings.TrimSpace(v.AsString())
		}
		if dir == "" {
			return "", "", fmt.Errorf("data file path unavailable for index %d", index)
		}
		rel := filepath.Clean(files[index].Path)
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
			return "", "", fmt.Errorf("unsafe relative path %q", files[index].Path)
		}
		frozen = filepath.Join(dir, rel)
		filename = filepath.Base(rel)
	}
	if filename == "" {
		filename = filepath.Base(frozen)
	}
	if err := validateDataFile(frozen, downloadRoots); err != nil {
		return "", "", err
	}
	return frozen, sanitizeTorrentFilename(filename), nil
}

func validateDataFile(path string, roots []string) error {
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
		return fmt.Errorf("data path %q is a symlink; refused", abs)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("data path %q is not a regular file", abs)
	}
	if st.Size() <= 0 {
		return fmt.Errorf("data path %q is empty", abs)
	}
	if st.Size() > maxDataFileBytes {
		return fmt.Errorf("data path %q exceeds %d byte limit", abs, maxDataFileBytes)
	}
	return nil
}
