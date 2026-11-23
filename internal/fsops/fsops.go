// Package fsops provides path-guarded filesystem helpers under download_roots.
package fsops

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	maxListEntries      = 2000
	maxTextBytes        = 512 << 10
	maxImageBytes       = 8 << 20
	maxCopyBytes        = 4 << 30
	minFingerprintBytes = 4 << 10
	maxFingerprintBytes = 256 << 10
)

// FileStat is path metadata for same-FS / hardlink checks.
type FileStat struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   string `json:"mode,omitempty"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
	Nlink  uint64 `json:"nlink"`
	IsDir  bool   `json:"isDir"`
}

// Entry is a directory listing row.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	Mode    string `json:"mode,omitempty"`
}

// Manager scopes all operations to AllowedRoots (no symlink traversal).
type Manager struct {
	AllowedRoots []string
	HideDot      bool
}

func (m *Manager) Roots() []string {
	out := make([]string, 0, len(m.AllowedRoots))
	for _, r := range m.AllowedRoots {
		r = filepath.Clean(strings.TrimSpace(r))
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

// Resolve checks a path is under allowed roots after symlink evaluation.
func (m *Manager) Resolve(path string, mustExist bool) (string, os.FileInfo, error) {
	roots := m.resolvedRoots()
	if len(roots) == 0 {
		return "", nil, fmt.Errorf("no download_roots configured; refuse path access")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil, fmt.Errorf("path required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	abs = filepath.Clean(abs)

	st, err := os.Lstat(abs)
	if err != nil {
		if !mustExist && os.IsNotExist(err) {
			parent := filepath.Dir(abs)
			pst, err := os.Lstat(parent)
			if err != nil {
				return "", nil, err
			}
			if pst.Mode()&os.ModeSymlink != 0 || !pst.IsDir() {
				return "", nil, fmt.Errorf("invalid parent directory")
			}
			resolvedParent, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return "", nil, fmt.Errorf("resolve parent: %w", err)
			}
			if !underRoots(resolvedParent, roots) {
				return "", nil, fmt.Errorf("path %q is outside allowed download_roots", abs)
			}
			candidate := filepath.Join(resolvedParent, filepath.Base(abs))
			if !underRoots(candidate, roots) {
				return "", nil, fmt.Errorf("path %q is outside allowed download_roots", abs)
			}
			return candidate, nil, nil
		}
		return "", nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return "", nil, fmt.Errorf("refusing symlink path")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", nil, fmt.Errorf("resolve path: %w", err)
	}
	if !underRoots(resolved, roots) {
		return "", nil, fmt.Errorf("path %q is outside allowed download_roots", abs)
	}
	return resolved, st, nil
}

func (m *Manager) resolvedRoots() []string {
	out := make([]string, 0, len(m.AllowedRoots))
	for _, r := range m.Roots() {
		resolved, err := evalOrAbs(r)
		if err != nil {
			continue
		}
		out = append(out, resolved)
	}
	return out
}

func evalOrAbs(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}

func underRoots(abs string, roots []string) bool {
	for _, root := range roots {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)) {
			return true
		}
	}
	return false
}

// List returns entries for a directory (or roots when path empty).
func (m *Manager) List(path string) ([]Entry, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		entries := make([]Entry, 0, len(m.Roots()))
		for _, root := range m.Roots() {
			st, err := os.Lstat(root)
			if err != nil {
				continue
			}
			entries = append(entries, Entry{
				Name:    filepath.Base(root),
				Path:    root,
				IsDir:   true,
				Size:    0,
				ModTime: st.ModTime().Unix(),
				Mode:    st.Mode().String(),
			})
		}
		return entries, "", nil
	}
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return nil, "", err
	}
	if !st.IsDir() {
		return nil, "", fmt.Errorf("not a directory")
	}
	dirents, err := os.ReadDir(abs)
	if err != nil {
		return nil, "", err
	}
	out := make([]Entry, 0, len(dirents))
	for _, d := range dirents {
		name := d.Name()
		if m.HideDot && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		full := filepath.Join(abs, name)
		out = append(out, Entry{
			Name:    name,
			Path:    full,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
			Mode:    info.Mode().String(),
		})
		if len(out) >= maxListEntries {
			break
		}
	}
	return out, abs, nil
}

// Mkdir creates a directory under roots.
func (m *Manager) Mkdir(path string) error {
	abs, _, err := m.Resolve(path, false)
	if err != nil {
		return err
	}
	return os.Mkdir(abs, 0o755)
}

// Rename renames within roots (same as move when target is new name).
// Falls back to copy+remove when rename fails across filesystems (EXDEV).
func (m *Manager) Rename(from, to string) error {
	src, st, err := m.Resolve(from, true)
	if err != nil {
		return err
	}
	_ = st
	dst, _, err := m.Resolve(to, false)
	if err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	if err := m.Copy(from, to); err != nil {
		return err
	}
	return m.Remove(from, true)
}

func isCrossDevice(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}

// Remove deletes a file or empty directory (recursive optional).
func (m *Manager) Remove(path string, recursive bool) error {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return err
	}
	// Refuse deleting a configured root itself.
	for _, root := range m.Roots() {
		if abs == root {
			return fmt.Errorf("refusing to remove download root")
		}
	}
	if st.IsDir() {
		if recursive {
			return os.RemoveAll(abs)
		}
		return os.Remove(abs)
	}
	return os.Remove(abs)
}

// Copy copies a regular file or directory tree within roots.
// Directory trees skip HideDot entries and refuse symlinks; total payload capped at maxCopyBytes.
func (m *Manager) Copy(from, to string) error {
	src, st, err := m.Resolve(from, true)
	if err != nil {
		return err
	}
	dst, _, err := m.Resolve(to, false)
	if err != nil {
		return err
	}
	var written int64
	if st.IsDir() {
		return m.copyDir(src, dst, &written)
	}
	return m.copyFile(src, dst, st.Size(), &written)
}

func (m *Manager) copyFile(src, dst string, size int64, written *int64) error {
	if size < 0 {
		size = 0
	}
	if *written+size > maxCopyBytes {
		return fmt.Errorf("copy exceeds size limit (%d bytes)", maxCopyBytes)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(in, maxCopyBytes+1))
	if err != nil {
		_ = os.Remove(dst)
		return err
	}
	*written += n
	if *written > maxCopyBytes {
		_ = os.Remove(dst)
		return fmt.Errorf("copy exceeds size limit (%d bytes)", maxCopyBytes)
	}
	return nil
}

func (m *Manager) copyDir(src, dst string, written *int64) error {
	if err := os.Mkdir(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if m.HideDot && strings.HasPrefix(name, ".") {
			continue
		}
		childFrom := filepath.Join(src, name)
		childTo := filepath.Join(dst, name)
		info, err := e.Info()
		if err != nil {
			_ = os.RemoveAll(dst)
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			_ = os.RemoveAll(dst)
			return fmt.Errorf("refusing to copy symlink %q", childFrom)
		}
		// Re-resolve every child so copy stays under download_roots.
		resolvedFrom, st, err := m.Resolve(childFrom, true)
		if err != nil {
			_ = os.RemoveAll(dst)
			return err
		}
		resolvedTo, _, err := m.Resolve(childTo, false)
		if err != nil {
			_ = os.RemoveAll(dst)
			return err
		}
		if st.IsDir() {
			if err := m.copyDir(resolvedFrom, resolvedTo, written); err != nil {
				_ = os.RemoveAll(dst)
				return err
			}
			continue
		}
		if err := m.copyFile(resolvedFrom, resolvedTo, st.Size(), written); err != nil {
			_ = os.RemoveAll(dst)
			return err
		}
	}
	return nil
}

// ReadText reads a small text file for NFO viewing (auto CP437 for .nfo when needed).
func (m *Manager) ReadText(path string) (string, error) {
	text, _, err := m.ReadTextEncoded(path, "")
	return text, err
}

// ReadTextEncoded reads a small text file with optional encoding hint (utf8|cp437|""=auto).
func (m *Manager) ReadTextEncoded(path, encodingHint string) (text, encoding string, err error) {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return "", "", err
	}
	if st.IsDir() {
		return "", "", fmt.Errorf("not a file")
	}
	if st.Size() > maxTextBytes {
		return "", "", fmt.Errorf("file too large for text view (max %d bytes)", maxTextBytes)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", "", err
	}
	text, encoding = DecodeTextBytes(path, raw, encodingHint)
	return text, encoding, nil
}

// ChecksumSHA256 returns hex SHA-256 of a regular file (size capped).
func (m *Manager) ChecksumSHA256(path string) (string, int64, error) {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return "", 0, err
	}
	if st.IsDir() {
		return "", 0, fmt.Errorf("not a file")
	}
	if st.Size() > maxCopyBytes {
		return "", 0, fmt.Errorf("file too large for checksum")
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, maxCopyBytes)); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), st.Size(), nil
}

// ReadBase64 returns a data URL for small image/binary previews.
func (m *Manager) ReadBase64(path string) (dataURL string, size int64, err error) {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return "", 0, err
	}
	if st.IsDir() {
		return "", 0, fmt.Errorf("not a file")
	}
	if st.Size() > maxImageBytes {
		return "", 0, fmt.Errorf("file too large for preview")
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", 0, err
	}
	mime := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	case ".svg":
		mime = "image/svg+xml"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), st.Size(), nil
}

// OpenFile opens a regular file for streaming download (caller closes).
func (m *Manager) OpenFile(path string) (*os.File, os.FileInfo, error) {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return nil, nil, err
	}
	if st.IsDir() {
		return nil, nil, fmt.Errorf("not a file")
	}
	if st.Size() > maxCopyBytes {
		return nil, nil, fmt.Errorf("file too large to download via FS API")
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	return f, st, nil
}

// Ensure within roots for share token paths.
func (m *Manager) MustFile(path string) (string, os.FileInfo, error) {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return "", nil, err
	}
	if st.IsDir() {
		return "", nil, fmt.Errorf("not a file")
	}
	return abs, st, nil
}

// Stat returns device/inode metadata for a path under roots.
func (m *Manager) Stat(path string) (FileStat, error) {
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return FileStat{}, err
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || sys == nil {
		return FileStat{}, fmt.Errorf("stat unavailable")
	}
	return FileStat{
		Path:   abs,
		Size:   st.Size(),
		Mode:   st.Mode().String(),
		Device: uint64(sys.Dev),
		Inode:  uint64(sys.Ino),
		Nlink:  uint64(sys.Nlink),
		IsDir:  st.IsDir(),
	}, nil
}

// Fingerprint returns size plus SHA-256 of the first and last nBytes of a file.
func (m *Manager) Fingerprint(path string, nBytes int) (size int64, digest string, err error) {
	if nBytes < minFingerprintBytes {
		nBytes = minFingerprintBytes
	}
	if nBytes > maxFingerprintBytes {
		nBytes = maxFingerprintBytes
	}
	abs, st, err := m.Resolve(path, true)
	if err != nil {
		return 0, "", err
	}
	if st.IsDir() {
		return 0, "", fmt.Errorf("not a file")
	}
	size = st.Size()
	f, err := os.Open(abs)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	firstN := int64(nBytes)
	if size < firstN {
		firstN = size
	}
	if firstN > 0 {
		if _, err := io.Copy(h, io.LimitReader(f, firstN)); err != nil {
			return 0, "", err
		}
	}
	if size > firstN {
		lastN := int64(nBytes)
		if size < lastN {
			lastN = size
		}
		off := size - lastN
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return 0, "", err
		}
		if _, err := io.Copy(h, io.LimitReader(f, lastN)); err != nil {
			return 0, "", err
		}
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

// Hardlink creates a new hard link at to pointing at from (both under roots).
// Fails if to already exists.
func (m *Manager) Hardlink(from, to string) error {
	src, st, err := m.Resolve(from, true)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("cannot hardlink a directory")
	}
	dst, _, err := m.Resolve(to, false)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("destination already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Link(src, dst)
}

// NowUnix is a tiny helper for tests.
func NowUnix() int64 { return time.Now().Unix() }
