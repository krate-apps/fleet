package buckets

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/sdk/manifest"
)

// Record is a configured bucket for this Fleet instance (BKT-001).
type Record struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Enabled   bool   `json:"enabled"`
	Official  bool   `json:"official,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// CatalogEntry is a discoverable extension from an enabled bucket (BKT-003).
type CatalogEntry struct {
	BucketID    string `json:"bucketId"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
	API         string `json:"api"`
	Origin      string `json:"origin"` // qualified bucket/id
	Path        string `json:"path,omitempty"`
	Repository  string `json:"repository,omitempty"`
	PackageURL  string `json:"packageUrl,omitempty"`
	Installable bool   `json:"installable"`
}

// ThemeCatalogEntry is a discoverable theme pack from an enabled bucket.
type ThemeCatalogEntry struct {
	BucketID    string `json:"bucketId"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
	Origin      string `json:"origin"`
	Path        string `json:"path,omitempty"`
	PackageURL  string `json:"packageUrl,omitempty"`
	Installable bool   `json:"installable"`
}

// Registry persists buckets and installs packages into the local extensions root.
type Registry struct {
	store        *storage.Store
	installRoot  string // where unpacked packages live (scanned by extension host)
	fleetDataDir string
}

// NewRegistry constructs a bucket registry.
func NewRegistry(store *storage.Store, fleetDataDir string) (*Registry, error) {
	installRoot := filepath.Join(fleetDataDir, "extensions-src")
	if err := os.MkdirAll(installRoot, 0o700); err != nil {
		return nil, err
	}
	r := &Registry{store: store, installRoot: installRoot, fleetDataDir: fleetDataDir}
	if err := r.migrate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) migrate() error {
	_, err := r.store.DB().Exec(`
CREATE TABLE IF NOT EXISTS buckets (
	id TEXT PRIMARY KEY NOT NULL,
	name TEXT NOT NULL,
	path TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	official INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);`)
	return err
}

// InstallRoot is the directory scanned for installed extension packages.
func (r *Registry) InstallRoot() string {
	return r.installRoot
}

// AddLocal registers a local bucket checkout (directory containing bucket.json).
func (r *Registry) AddLocal(path string) (*Record, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	m, err := LoadBucketDir(abs)
	if err != nil {
		return nil, fmt.Errorf("load bucket: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = r.store.DB().Exec(`
INSERT INTO buckets(id, name, path, enabled, official, updated_at)
VALUES(?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
  name=excluded.name,
  path=excluded.path,
  official=excluded.official,
  updated_at=excluded.updated_at
`, m.ID, m.Name, abs, 1, boolInt(m.Official), now)
	if err != nil {
		return nil, err
	}
	return r.Get(m.ID)
}

// List returns configured buckets.
func (r *Registry) List() ([]Record, error) {
	rows, err := r.store.DB().Query(`SELECT id, name, path, enabled, official, updated_at FROM buckets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Record, 0)
	for rows.Next() {
		var rec Record
		var en, off int
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Path, &en, &off, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		rec.Enabled = en != 0
		rec.Official = off != 0
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Get returns one bucket.
func (r *Registry) Get(id string) (*Record, error) {
	var rec Record
	var en, off int
	err := r.store.DB().QueryRow(
		`SELECT id, name, path, enabled, official, updated_at FROM buckets WHERE id = ?`, id,
	).Scan(&rec.ID, &rec.Name, &rec.Path, &en, &off, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bucket not found")
	}
	if err != nil {
		return nil, err
	}
	rec.Enabled = en != 0
	rec.Official = off != 0
	return &rec, nil
}

// SetEnabled activates or disables a bucket (BKT-001).
func (r *Registry) SetEnabled(id string, enabled bool) (*Record, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.store.DB().Exec(
		`UPDATE buckets SET enabled = ?, updated_at = ? WHERE id = ?`, boolInt(enabled), now, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("bucket not found")
	}
	return r.Get(id)
}

// Remove deletes a bucket registration (does not uninstall packages).
func (r *Registry) Remove(id string) error {
	res, err := r.store.DB().Exec(`DELETE FROM buckets WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("bucket not found")
	}
	return nil
}

// Catalog lists extensions from enabled buckets (BKT-003).
func (r *Registry) Catalog() ([]CatalogEntry, error) {
	buckets, err := r.List()
	if err != nil {
		return nil, err
	}
	out := make([]CatalogEntry, 0)
	for _, b := range buckets {
		if !b.Enabled {
			continue
		}
		listings, err := ListBucketExtensions(b.Path)
		if err != nil {
			return nil, fmt.Errorf("bucket %s: %w", b.ID, err)
		}
		for _, l := range listings {
			origin := b.ID + "/" + l.ID
			path := ""
			repo := ""
			if l.Source != nil {
				path = strings.TrimSpace(l.Source.Path)
				repo = strings.TrimSpace(l.Source.Repository)
			}
			pkgURL := l.PackageURL()
			pkgSHA := l.PackageSHA256()
			if repo == "" {
				repo = strings.TrimSpace(l.Homepage)
			}
			installable := false
			if path != "" {
				rel := filepath.Clean(path)
				if !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
					if packageDirHasManifest(filepath.Join(b.Path, rel)) {
						installable = true
					}
				}
			}
			if !installable && pkgURL != "" {
				if localArchiveForPackage(b.Path, pkgURL) != "" {
					installable = true
				} else if pkgSHA != "" {
					installable = true
				}
			}
			out = append(out, CatalogEntry{
				BucketID:    b.ID,
				ID:          l.ID,
				Name:        l.Name,
				Description: strings.TrimSpace(l.Description),
				Version:     l.Version,
				API:         l.API,
				Origin:      origin,
				Path:        path,
				Repository:  repo,
				PackageURL:  pkgURL,
				Installable: installable,
			})
		}
	}
	return out, nil
}

// ThemesCatalog lists theme packs from enabled buckets.
func (r *Registry) ThemesCatalog() ([]ThemeCatalogEntry, error) {
	buckets, err := r.List()
	if err != nil {
		return nil, err
	}
	out := make([]ThemeCatalogEntry, 0)
	for _, b := range buckets {
		if !b.Enabled {
			continue
		}
		listings, err := ListBucketThemes(b.Path)
		if err != nil {
			return nil, fmt.Errorf("bucket %s: %w", b.ID, err)
		}
		for _, l := range listings {
			origin := b.ID + "/" + l.ID
			path := ""
			if l.Source != nil {
				path = strings.TrimSpace(l.Source.Path)
			}
			pkgURL := l.PackageURL()
			pkgSHA := l.PackageSHA256()
			installable := false
			if path != "" {
				rel := filepath.Clean(path)
				if !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
					if themePackageDirHasManifest(filepath.Join(b.Path, rel)) {
						installable = true
					}
				}
			}
			if !installable && pkgURL != "" {
				if localArchiveForPackage(b.Path, pkgURL) != "" {
					installable = true
				} else if pkgSHA != "" {
					installable = true
				}
			}
			out = append(out, ThemeCatalogEntry{
				BucketID:    b.ID,
				ID:          l.ID,
				Name:        l.Name,
				Description: strings.TrimSpace(l.Description),
				Version:     l.Version,
				Origin:      origin,
				Path:        path,
				PackageURL:  pkgURL,
				Installable: installable,
			})
		}
	}
	return out, nil
}

// ResolveThemePackageDir returns the on-disk theme package directory for a bucket listing.
func (r *Registry) ResolveThemePackageDir(bucketID, themeID string) (string, error) {
	b, err := r.Get(bucketID)
	if err != nil {
		return "", err
	}
	if !b.Enabled {
		return "", fmt.Errorf("bucket disabled")
	}
	listings, err := ListBucketThemes(b.Path)
	if err != nil {
		return "", err
	}
	var listing *ThemeListing
	for i := range listings {
		if listings[i].ID == themeID {
			listing = &listings[i]
			break
		}
	}
	if listing == nil {
		return "", fmt.Errorf("theme not in bucket catalog")
	}
	if listing.Source != nil && strings.TrimSpace(listing.Source.Path) != "" {
		rel := filepath.Clean(listing.Source.Path)
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", fmt.Errorf("invalid source.path")
		}
		local := filepath.Join(b.Path, rel)
		if themePackageDirHasManifest(local) {
			return local, nil
		}
	}
	if pkgURL := listing.PackageURL(); pkgURL != "" {
		if archive := localArchiveForPackage(b.Path, pkgURL); archive != "" {
			return "", fmt.Errorf("theme archive install not wired via ResolveThemePackageDir; use package path")
		}
	}
	return "", fmt.Errorf("listing has neither usable source.path nor local theme package")
}

// InstallLocal copies a package directory into the install root after validating fleet.ext.yaml (BKT-005 local).
func (r *Registry) InstallLocal(srcDir string) (*manifest.Manifest, error) {
	abs, err := filepath.Abs(strings.TrimSpace(srcDir))
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(abs, "fleet.ext.yaml"))
	if err != nil {
		// try alternate names
		for _, name := range []string{"extension.yaml", "manifest.yaml"} {
			raw, err = os.ReadFile(filepath.Join(abs, name))
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("extension manifest: %w", err)
		}
	}
	m, err := manifest.ParseYAML(raw)
	if err != nil {
		return nil, err
	}
	dest := filepath.Join(r.installRoot, safeDirName(m.ID))
	if err := os.RemoveAll(dest); err != nil {
		return nil, err
	}
	if err := copyDir(abs, dest); err != nil {
		_ = os.RemoveAll(dest)
		return nil, fmt.Errorf("install copy: %w", err)
	}
	return m, nil
}

func safeDirName(id string) string {
	return strings.ReplaceAll(id, "/", "_")
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
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
		// Reject path traversal and symlinks (BKT-010 lite).
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink rejected: %s", rel)
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		mode := info.Mode().Perm()
		if mode&0o111 != 0 {
			mode = 0o700
		} else {
			mode = 0o600
		}
		return copyFile(path, target, mode)
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
