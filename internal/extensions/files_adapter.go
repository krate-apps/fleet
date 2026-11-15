package extensions

import (
	"github.com/krate-apps/fleet/internal/fsops"
)

// FilesAdapter adapts fsops.Manager to FilesControl.
type FilesAdapter struct {
	M *fsops.Manager
}

func (a FilesAdapter) Roots() []string {
	if a.M == nil {
		return nil
	}
	return a.M.Roots()
}

func (a FilesAdapter) List(path string) ([]map[string]any, string, error) {
	entries, cwd, err := a.M.List(path)
	if err != nil {
		return nil, "", err
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{
			"name":    e.Name,
			"path":    e.Path,
			"isDir":   e.IsDir,
			"size":    e.Size,
			"modTime": e.ModTime,
			"mode":    e.Mode,
		})
	}
	return out, cwd, nil
}

func (a FilesAdapter) Mkdir(path string) error { return a.M.Mkdir(path) }

func (a FilesAdapter) Rename(from, to string) error { return a.M.Rename(from, to) }

func (a FilesAdapter) Remove(path string, recursive bool) error {
	return a.M.Remove(path, recursive)
}

func (a FilesAdapter) Copy(from, to string) error { return a.M.Copy(from, to) }

func (a FilesAdapter) Move(from, to string) error { return a.M.Rename(from, to) }

func (a FilesAdapter) ReadText(path string) (string, error) { return a.M.ReadText(path) }

func (a FilesAdapter) ReadBase64(path string) (string, int64, error) {
	return a.M.ReadBase64(path)
}

func (a FilesAdapter) Checksum(path string) (string, string, int64, error) {
	h, n, err := a.M.ChecksumSHA256(path)
	return "sha256", h, n, err
}

func (a FilesAdapter) Stat(path string) (map[string]any, error) {
	st, err := a.M.Stat(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"path":   st.Path,
		"size":   st.Size,
		"mode":   st.Mode,
		"device": st.Device,
		"inode":  st.Inode,
		"nlink":  st.Nlink,
		"isDir":  st.IsDir,
	}, nil
}

func (a FilesAdapter) Fingerprint(path string, nBytes int) (int64, string, error) {
	return a.M.Fingerprint(path, nBytes)
}

func (a FilesAdapter) Hardlink(from, to string) error {
	return a.M.Hardlink(from, to)
}
