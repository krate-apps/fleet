// Package themes discovers installed theme packs under data_dir/themes-src.
package themes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/krate-apps/fleet/sdk/theme"
)

// Installed is a theme pack ready for the UI.
type Installed struct {
	Manifest *theme.Manifest
	Dir      string
	CSSPath  string
}

// Store scans themes-src.
type Store struct {
	Root string // typically filepath.Join(dataDir, "themes-src")
}

// List returns valid installed theme packs (one directory per pack).
func (s *Store) List() ([]Installed, error) {
	if s == nil || strings.TrimSpace(s.Root) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Installed, 0)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(s.Root, ent.Name())
		inst, err := loadDir(dir)
		if err != nil {
			continue
		}
		out = append(out, *inst)
	}
	return out, nil
}

// GetByShortID finds a pack by data-fleet-theme id (e.g. slate).
func (s *Store) GetByShortID(shortID string) (*Installed, error) {
	shortID = strings.TrimSpace(shortID)
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Manifest.ShortID() == shortID || all[i].Manifest.ID == shortID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("theme not found: %s", shortID)
}

// InstallLocal copies a theme package directory into the store root.
func (s *Store) InstallLocal(srcDir string) (*Installed, error) {
	if s == nil || strings.TrimSpace(s.Root) == "" {
		return nil, fmt.Errorf("themes store root required")
	}
	inst, err := loadDir(srcDir)
	if err != nil {
		return nil, err
	}
	destName := safeDirName(inst.Manifest.ID)
	dest := filepath.Join(s.Root, destName)
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return nil, err
	}
	_ = os.RemoveAll(dest)
	if err := copyDir(srcDir, dest); err != nil {
		return nil, err
	}
	return loadDir(dest)
}

// Uninstall removes an installed theme pack by short id or package id.
// Built-in dark/light are not on disk and cannot be uninstalled here.
func (s *Store) Uninstall(id string) error {
	if s == nil || strings.TrimSpace(s.Root) == "" {
		return fmt.Errorf("themes store root required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("theme id required")
	}
	switch strings.ToLower(id) {
	case "dark", "light":
		return fmt.Errorf("built-in theme cannot be uninstalled")
	}
	inst, err := s.GetByShortID(id)
	if err != nil {
		// Also try directory named after full id.
		dest := filepath.Join(s.Root, safeDirName(id))
		st, statErr := os.Stat(dest)
		if statErr != nil || !st.IsDir() {
			return err
		}
		return os.RemoveAll(dest)
	}
	return os.RemoveAll(inst.Dir)
}

func loadDir(dir string) (*Installed, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "fleet.theme.yaml"))
	if err != nil {
		return nil, err
	}
	m, err := theme.ParseYAML(raw)
	if err != nil {
		return nil, err
	}
	cssPath := filepath.Join(dir, filepath.Clean(m.CSS))
	if !strings.HasPrefix(cssPath, filepath.Clean(dir)+string(os.PathSeparator)) && cssPath != filepath.Clean(dir) {
		return nil, fmt.Errorf("css escapes package dir")
	}
	if _, err := os.Stat(cssPath); err != nil {
		return nil, fmt.Errorf("css missing: %w", err)
	}
	return &Installed{Manifest: m, Dir: dir, CSSPath: cssPath}, nil
}

func safeDirName(id string) string {
	return strings.ReplaceAll(id, "/", "_")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o700)
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not allowed: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}
