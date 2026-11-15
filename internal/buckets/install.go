package buckets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/krate-apps/fleet/sdk/manifest"
)

// InstallArchive unpacks a local .tar.gz (optional sha256) into the install root (BKT-005/010).
func (r *Registry) InstallArchive(archivePath, sha256sum string) (*manifest.Manifest, error) {
	abs, err := filepath.Abs(strings.TrimSpace(archivePath))
	if err != nil {
		return nil, err
	}
	if sha256sum != "" {
		if err := VerifySHA256(abs, sha256sum); err != nil {
			return nil, err
		}
	}
	tmp, err := os.MkdirTemp(r.fleetDataDir, "fleet-extract-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := ExtractTarGz(abs, tmp); err != nil {
		return nil, err
	}
	root, err := FindPackageRoot(tmp)
	if err != nil {
		return nil, err
	}
	return r.InstallLocal(root)
}

// InstallURL downloads a package archive then installs it (BKT-004).
func (r *Registry) InstallURL(ctx context.Context, rawURL, sha256sum string, opts DownloadOptions) (*manifest.Manifest, error) {
	if strings.TrimSpace(sha256sum) == "" {
		return nil, fmt.Errorf("sha256 is required for remote installs")
	}
	norm, err := NormalizePackageURL(rawURL)
	if err != nil {
		return nil, err
	}
	// GitHub hosts must use a pinned Releases asset URL (BKT-004 diagnostic).
	if hostLooksLikeGitHub(norm) {
		if err := RecognizeGitHubPackageURL(norm); err != nil {
			return nil, err
		}
	}
	tmpDir, err := os.MkdirTemp(r.fleetDataDir, "fleet-dl-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, "package.tar.gz")
	if err := r.FetchToFile(ctx, norm, archivePath, opts); err != nil {
		return nil, err
	}
	return r.InstallArchive(archivePath, sha256sum)
}

func hostLooksLikeGitHub(raw string) bool {
	u, err := validatePackageURL(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Host)
	return h == "github.com" || h == "www.github.com"
}

// InstallFromBucket prefers local source.path, else architecture url+hash.
func (r *Registry) InstallFromBucket(bucketID, extensionID string) (*manifest.Manifest, error) {
	return r.InstallFromBucketContext(context.Background(), bucketID, extensionID, DownloadOptions{})
}

// UninstallPackage removes an installed extension directory from the install root (BKT-006).
func (r *Registry) UninstallPackage(extensionID string) error {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return fmt.Errorf("extension id required")
	}
	dest := filepath.Join(r.installRoot, safeDirName(id))
	st, err := os.Stat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("extension not installed at %s", dest)
		}
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("install path is not a directory: %s", dest)
	}
	return os.RemoveAll(dest)
}

// InstallFromBucketContext prefers a present source.path, then a mirrored archives/
// asset (private/offline installs), then architecture url+hash.
func (r *Registry) InstallFromBucketContext(ctx context.Context, bucketID, extensionID string, opts DownloadOptions) (*manifest.Manifest, error) {
	b, err := r.Get(bucketID)
	if err != nil {
		return nil, err
	}
	if !b.Enabled {
		return nil, fmt.Errorf("bucket disabled")
	}
	listings, err := ListBucketExtensions(b.Path)
	if err != nil {
		return nil, err
	}
	var listing *ExtensionListing
	for i := range listings {
		if listings[i].ID == extensionID {
			listing = &listings[i]
			break
		}
	}
	if listing == nil {
		return nil, fmt.Errorf("extension not in bucket catalog")
	}
	if listing.Source != nil && strings.TrimSpace(listing.Source.Path) != "" {
		rel := filepath.Clean(listing.Source.Path)
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("invalid source.path")
		}
		local := filepath.Join(b.Path, rel)
		if packageDirHasManifest(local) {
			return r.InstallLocal(local)
		}
	}
	pkgURL := listing.PackageURL()
	pkgSHA := listing.PackageSHA256()
	if pkgURL != "" {
		if archive := localArchiveForPackage(b.Path, pkgURL); archive != "" {
			return r.InstallArchive(archive, pkgSHA)
		}
		if pkgSHA == "" {
			return nil, fmt.Errorf("architecture hash is required for remote installs")
		}
		return r.InstallURL(ctx, pkgURL, pkgSHA, opts)
	}
	return nil, fmt.Errorf("listing has neither usable source.path nor architecture url")
}

func packageDirHasManifest(dir string) bool {
	for _, name := range []string{"fleet.ext.yaml", "extension.yaml", "manifest.yaml"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// localArchiveForPackage looks for a release asset mirrored under bucket/archives/.
func localArchiveForPackage(bucketPath, pkgURL string) string {
	base := filepath.Base(strings.TrimSpace(pkgURL))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	cand := filepath.Join(bucketPath, "archives", base)
	st, err := os.Stat(cand)
	if err != nil || st.IsDir() {
		return ""
	}
	return cand
}
