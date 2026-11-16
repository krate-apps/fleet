package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths holds configured helper binary locations (empty = PATH / unset).
type Paths struct {
	Mktorrent   string `json:"mktorrent"`
	Dumptorrent string `json:"dumptorrent"`
	Mediainfo   string `json:"mediainfo"`
	Ffmpeg      string `json:"ffmpeg"`
	Sox         string `json:"sox"`
	Unrar       string `json:"unrar"`
	Rclone      string `json:"rclone"`
}

// Normalize trims spaces on every path field.
func (p Paths) Normalize() Paths {
	p.Mktorrent = strings.TrimSpace(p.Mktorrent)
	p.Dumptorrent = strings.TrimSpace(p.Dumptorrent)
	p.Mediainfo = strings.TrimSpace(p.Mediainfo)
	p.Ffmpeg = strings.TrimSpace(p.Ffmpeg)
	p.Sox = strings.TrimSpace(p.Sox)
	p.Unrar = strings.TrimSpace(p.Unrar)
	p.Rclone = strings.TrimSpace(p.Rclone)
	return p
}

// OverlayNonEmpty returns base with non-empty fields from over written on top.
func OverlayNonEmpty(base, over Paths) Paths {
	base = base.Normalize()
	over = over.Normalize()
	if over.Mktorrent != "" {
		base.Mktorrent = over.Mktorrent
	}
	if over.Dumptorrent != "" {
		base.Dumptorrent = over.Dumptorrent
	}
	if over.Mediainfo != "" {
		base.Mediainfo = over.Mediainfo
	}
	if over.Ffmpeg != "" {
		base.Ffmpeg = over.Ffmpeg
	}
	if over.Sox != "" {
		base.Sox = over.Sox
	}
	if over.Unrar != "" {
		base.Unrar = over.Unrar
	}
	if over.Rclone != "" {
		base.Rclone = over.Rclone
	}
	return base
}

// OverridesOnly keeps only non-empty paths (SQLite overlay payload).
func OverridesOnly(p Paths) Paths {
	p = p.Normalize()
	var out Paths
	if p.Mktorrent != "" {
		out.Mktorrent = p.Mktorrent
	}
	if p.Dumptorrent != "" {
		out.Dumptorrent = p.Dumptorrent
	}
	if p.Mediainfo != "" {
		out.Mediainfo = p.Mediainfo
	}
	if p.Ffmpeg != "" {
		out.Ffmpeg = p.Ffmpeg
	}
	if p.Sox != "" {
		out.Sox = p.Sox
	}
	if p.Unrar != "" {
		out.Unrar = p.Unrar
	}
	if p.Rclone != "" {
		out.Rclone = p.Rclone
	}
	return out
}

// ValidatePath checks a configured binary path (empty is allowed).
func ValidatePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute: %s", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() || st.Mode()&0o111 == 0 {
		return fmt.Errorf("not an executable file: %s", path)
	}
	return nil
}

// ValidatePaths validates every non-empty path in p.
func ValidatePaths(p Paths) error {
	p = p.Normalize()
	for _, pair := range []struct {
		name, path string
	}{
		{"mktorrent", p.Mktorrent},
		{"dumptorrent", p.Dumptorrent},
		{"mediainfo", p.Mediainfo},
		{"ffmpeg", p.Ffmpeg},
		{"sox", p.Sox},
		{"unrar", p.Unrar},
		{"rclone", p.Rclone},
	} {
		if err := ValidatePath(pair.path); err != nil {
			return fmt.Errorf("%s: %w", pair.name, err)
		}
	}
	return nil
}

// Paths returns a snapshot of configured helper paths (thread-safe).
func (r *Runner) Paths() Paths {
	if r == nil {
		return Paths{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Paths{
		Mktorrent:   r.MktorrentPath,
		Dumptorrent: r.DumptorrentPath,
		Mediainfo:   r.MediainfoPath,
		Ffmpeg:      r.FfmpegPath,
		Sox:         r.SoxPath,
		Unrar:       r.UnrarPath,
		Rclone:      r.RclonePath,
	}.Normalize()
}

// SetPaths updates helper paths atomically (AllowedRoots / ArtifactsDir unchanged).
func (r *Runner) SetPaths(p Paths) {
	if r == nil {
		return
	}
	p = p.Normalize()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.MktorrentPath = p.Mktorrent
	r.DumptorrentPath = p.Dumptorrent
	r.MediainfoPath = p.Mediainfo
	r.FfmpegPath = p.Ffmpeg
	r.SoxPath = p.Sox
	r.UnrarPath = p.Unrar
	r.RclonePath = p.Rclone
}

func (r *Runner) lockedPath(get func(*Runner) string) string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return strings.TrimSpace(get(r))
}

func resolveNamed(configured, fallbackName string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}
	if fallbackName == "" {
		return ""
	}
	if p, err := exec.LookPath(fallbackName); err == nil {
		return p
	}
	return ""
}

func toolStatusEntry(configured, fallbackName string) map[string]any {
	configured = strings.TrimSpace(configured)
	resolved := resolveNamed(configured, fallbackName)
	out := map[string]any{
		"path":      configured,
		"resolved":  resolved,
		"available": false,
	}
	check := resolved
	if check == "" {
		out["message"] = "not configured"
		return out
	}
	st, err := os.Stat(check)
	if err != nil {
		out["message"] = err.Error()
		return out
	}
	if st.IsDir() || st.Mode()&0o111 == 0 {
		out["message"] = "not an executable file"
		return out
	}
	out["available"] = true
	return out
}
