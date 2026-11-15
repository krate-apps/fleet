package buckets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const SchemaVersion = 1

// DefaultArchitecture is the preferred package key in architecture maps.
const DefaultArchitecture = "linux-amd64"

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)

// BucketManifest is bucket.json (bucket root).
type BucketManifest struct {
	Schema      int    `json:"schema"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Official    bool   `json:"official,omitempty"`
}

// ArchArtifact is an architecture entry (url + hash).
type ArchArtifact struct {
	URL  string `json:"url,omitempty"`
	Hash string `json:"hash,omitempty"`
}

// ListingSource is an optional local checkout path inside the bucket.
type ListingSource struct {
	Repository string `json:"repository,omitempty"`
	Path       string `json:"path,omitempty"`
}

// ExtensionListing is bucket/<key>.json with kind "extension".
type ExtensionListing struct {
	Schema       int                     `json:"schema"`
	Kind         string                  `json:"kind,omitempty"`
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description,omitempty"`
	Version      string                  `json:"version"`
	API          string                  `json:"api"`
	Homepage     string                  `json:"homepage,omitempty"`
	License      string                  `json:"license,omitempty"`
	Architecture map[string]ArchArtifact `json:"architecture,omitempty"`
	Checkver     json.RawMessage         `json:"checkver,omitempty"`
	Autoupdate   json.RawMessage         `json:"autoupdate,omitempty"`
	Source       *ListingSource          `json:"source,omitempty"`
}

// ThemeListing is bucket/<key>.json with kind "theme".
type ThemeListing struct {
	Schema       int                     `json:"schema"`
	Kind         string                  `json:"kind,omitempty"`
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description,omitempty"`
	Version      string                  `json:"version"`
	Homepage     string                  `json:"homepage,omitempty"`
	License      string                  `json:"license,omitempty"`
	Architecture map[string]ArchArtifact `json:"architecture,omitempty"`
	Checkver     json.RawMessage         `json:"checkver,omitempty"`
	Autoupdate   json.RawMessage         `json:"autoupdate,omitempty"`
	Source       *ListingSource          `json:"source,omitempty"`
}

// NormalizeHash accepts "sha256:<hex>" or bare hex and returns lowercase hex (or empty).
func NormalizeHash(raw string) string {
	h := strings.TrimSpace(raw)
	if h == "" {
		return ""
	}
	if i := strings.IndexByte(h, ':'); i >= 0 {
		algo := strings.ToLower(strings.TrimSpace(h[:i]))
		h = strings.TrimSpace(h[i+1:])
		if algo != "" && algo != "sha256" {
			return ""
		}
	}
	h = strings.ToLower(h)
	if len(h) != 64 {
		return h // leave non-64 for VerifySHA256 to reject
	}
	for _, c := range h {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return h
}

func preferArch(arch map[string]ArchArtifact) ArchArtifact {
	if arch == nil {
		return ArchArtifact{}
	}
	if a, ok := arch[DefaultArchitecture]; ok {
		return a
	}
	for _, a := range arch {
		return a
	}
	return ArchArtifact{}
}

// PackageURL returns the preferred architecture download URL.
func (m *ExtensionListing) PackageURL() string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(preferArch(m.Architecture).URL)
}

// PackageSHA256 returns the preferred architecture hash as bare hex.
func (m *ExtensionListing) PackageSHA256() string {
	if m == nil {
		return ""
	}
	return NormalizeHash(preferArch(m.Architecture).Hash)
}

// PackageURL returns the preferred architecture download URL.
func (m *ThemeListing) PackageURL() string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(preferArch(m.Architecture).URL)
}

// PackageSHA256 returns the preferred architecture hash as bare hex.
func (m *ThemeListing) PackageSHA256() string {
	if m == nil {
		return ""
	}
	return NormalizeHash(preferArch(m.Architecture).Hash)
}

// ParseBucketJSON validates a bucket root manifest.
func ParseBucketJSON(data []byte) (*BucketManifest, error) {
	var m BucketManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse bucket: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks bucket root constraints.
func (m *BucketManifest) Validate() error {
	if m == nil {
		return fmt.Errorf("bucket manifest is nil")
	}
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported bucket schema %d", m.Schema)
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("invalid bucket id %q", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("bucket name required")
	}
	return nil
}

// ParseExtensionListingJSON validates a bucket extension entry.
func ParseExtensionListingJSON(data []byte) (*ExtensionListing, error) {
	var m ExtensionListing
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse listing: %w", err)
	}
	if m.Kind == "" {
		m.Kind = "extension"
	}
	if m.Kind != "extension" {
		return nil, fmt.Errorf("listing kind must be extension (got %q)", m.Kind)
	}
	if m.Schema != SchemaVersion {
		return nil, fmt.Errorf("unsupported listing schema %d", m.Schema)
	}
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("listing id, name, version required")
	}
	if len(m.Description) > 280 {
		return nil, fmt.Errorf("listing description must be <= 280 characters")
	}
	if strings.TrimSpace(m.API) == "" {
		return nil, fmt.Errorf("listing api required")
	}
	return &m, nil
}

// ParseThemeListingJSON validates a bucket theme entry.
func ParseThemeListingJSON(data []byte) (*ThemeListing, error) {
	var m ThemeListing
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse theme listing: %w", err)
	}
	if m.Kind == "" {
		m.Kind = "theme"
	}
	if m.Kind != "theme" {
		return nil, fmt.Errorf("listing kind must be theme (got %q)", m.Kind)
	}
	if m.Schema != SchemaVersion {
		return nil, fmt.Errorf("unsupported theme listing schema %d", m.Schema)
	}
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("theme listing id, name, version required")
	}
	if len(m.Description) > 280 {
		return nil, fmt.Errorf("theme listing description must be <= 280 characters")
	}
	return &m, nil
}

// LoadBucketDir reads bucket.json from a directory.
func LoadBucketDir(dir string) (*BucketManifest, error) {
	path := filepath.Join(dir, "bucket.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if _, err2 := os.Stat(filepath.Join(dir, "bucket.yaml")); err2 == nil {
				return nil, fmt.Errorf("bucket.yaml is no longer supported; use bucket.json")
			}
		}
		return nil, err
	}
	return ParseBucketJSON(raw)
}

type appListingKind struct {
	Kind string `json:"kind"`
}

// ListBucketExtensions reads bucket/*.json listings with kind extension.
func ListBucketExtensions(dir string) ([]ExtensionListing, error) {
	return listBucketExtensions(dir)
}

func listBucketExtensions(dir string) ([]ExtensionListing, error) {
	appDir := filepath.Join(dir, "bucket")
	entries, err := os.ReadDir(appDir)
	if err != nil {
		if os.IsNotExist(err) {
			if staleYAMLTree(dir, "extensions") {
				return nil, fmt.Errorf("extensions/*.yaml is no longer supported; use bucket/*.json")
			}
			return nil, nil
		}
		return nil, err
	}
	out := make([]ExtensionListing, 0)
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") || strings.HasSuffix(ent.Name(), ".json.template") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(appDir, ent.Name()))
		if err != nil {
			return nil, err
		}
		var kind appListingKind
		_ = json.Unmarshal(raw, &kind)
		k := strings.TrimSpace(kind.Kind)
		if k == "" {
			k = "extension"
		}
		if k != "extension" {
			continue
		}
		listing, err := ParseExtensionListingJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
		out = append(out, *listing)
	}
	return out, nil
}

// ListBucketThemes reads bucket/*.json listings with kind theme.
func ListBucketThemes(dir string) ([]ThemeListing, error) {
	appDir := filepath.Join(dir, "bucket")
	entries, err := os.ReadDir(appDir)
	if err != nil {
		if os.IsNotExist(err) {
			if staleYAMLTree(dir, "themes") {
				return nil, fmt.Errorf("themes/*.yaml is no longer supported; use bucket/*.json")
			}
			return nil, nil
		}
		return nil, err
	}
	out := make([]ThemeListing, 0)
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") || strings.HasSuffix(ent.Name(), ".json.template") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(appDir, ent.Name()))
		if err != nil {
			return nil, err
		}
		var kind appListingKind
		if err := json.Unmarshal(raw, &kind); err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
		if strings.TrimSpace(kind.Kind) != "theme" {
			continue
		}
		listing, err := ParseThemeListingJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
		out = append(out, *listing)
	}
	return out, nil
}

func staleYAMLTree(dir, sub string) bool {
	entries, err := os.ReadDir(filepath.Join(dir, sub))
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if !ent.IsDir() && strings.HasSuffix(ent.Name(), ".yaml") {
			return true
		}
	}
	return false
}
