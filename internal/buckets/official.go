package buckets

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OfficialBucketID is the reserved id for the preconfigured Fleet bucket (BKT-002).
const OfficialBucketID = "org.fleet.official"

// EnsureOfficialOptions controls official bucket seeding at startup.
type EnsureOfficialOptions struct {
	// Path is the on-disk bucket directory (bucket.json). Empty skips seeding.
	Path string
	// Disabled skips registration entirely (Fleet runs with zero buckets).
	Disabled bool
}

// EnsureOfficialBucket registers the official bucket once when present and not disabled.
// Existing registrations (including user-disabled) are left untouched.
func (r *Registry) EnsureOfficialBucket(opts EnsureOfficialOptions) (*Record, error) {
	if opts.Disabled {
		return nil, nil
	}
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("official bucket path is not a directory")
	}
	m, err := LoadBucketDir(abs)
	if err != nil {
		return nil, fmt.Errorf("official bucket: %w", err)
	}
	if m.ID != OfficialBucketID {
		return nil, fmt.Errorf("official bucket id must be %s (got %s)", OfficialBucketID, m.ID)
	}

	var existing string
	err = r.store.DB().QueryRow(`SELECT id FROM buckets WHERE id = ?`, OfficialBucketID).Scan(&existing)
	if err == nil {
		return r.Get(OfficialBucketID)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = r.store.DB().Exec(`
INSERT INTO buckets(id, name, path, enabled, official, updated_at)
VALUES(?,?,?,?,?,?)
`, m.ID, m.Name, abs, 1, 1, now)
	if err != nil {
		return nil, err
	}
	return r.Get(OfficialBucketID)
}

// SyncOfficialLocalPackages copies reference extension trees into the official
// bucket packages/ directory when a source checkout is available.
func SyncOfficialLocalPackages(bucketPath string, extensionRoots ...string) error {
	names := []string{"rssfeeds", "automove", "scheduler", "ratio", "archiveextract"}
	for _, name := range names {
		dst := filepath.Join(bucketPath, "packages", name)
		if st, err := os.Stat(filepath.Join(dst, "fleet.ext.yaml")); err == nil && !st.IsDir() {
			continue
		}
		var src string
		for _, root := range extensionRoots {
			cand := filepath.Join(root, name)
			if st, err := os.Stat(filepath.Join(cand, "fleet.ext.yaml")); err == nil && !st.IsDir() {
				src = cand
				break
			}
		}
		if src == "" {
			continue
		}
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("sync %s: %w", name, err)
		}
	}
	return SyncOfficialThemePackages(bucketPath)
}

// SyncOfficialThemePackages copies installable theme packs into
// bucket/packages/themes/<id> when a workspace theme checkout is present.
func SyncOfficialThemePackages(bucketPath string) error {
	ids := []string{"slate", "ocean", "sand"}
	absBucket, err := filepath.Abs(bucketPath)
	if err != nil {
		return err
	}
	parent := filepath.Dir(absBucket) // …/buckets or …/packaging
	for _, id := range ids {
		dst := filepath.Join(absBucket, "packages", "themes", id)
		if themePackageDirHasManifest(dst) {
			continue
		}
		// Workspace layouts: fleet/buckets/… or install-staged packaging/….
		candidates := []string{
			filepath.Join(filepath.Dir(parent), "themes", "fleet-theme-"+id),
			filepath.Join(filepath.Dir(filepath.Dir(parent)), "themes", "fleet-theme-"+id),
			filepath.Join(parent, "fleet-theme-"+id),
			filepath.Join(filepath.Dir(parent), "fleet-theme-"+id),
		}
		var src string
		for _, cand := range candidates {
			if cand == dst {
				continue
			}
			if themePackageDirHasManifest(cand) {
				src = cand
				break
			}
		}
		if src == "" {
			continue
		}
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("sync theme %s: %w", id, err)
		}
	}
	return nil
}

func themePackageDirHasManifest(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "fleet.theme.yaml"))
	return err == nil && !st.IsDir()
}

// ResolveOfficialBucketPath picks the first existing candidate directory.
func ResolveOfficialBucketPath(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if st, err := os.Stat(filepath.Join(abs, "bucket.json")); err == nil && !st.IsDir() {
			return abs
		}
	}
	return ""
}
