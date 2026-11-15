package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// AvailableRelease is a GitHub release entry with a resolved core asset (when found).
type AvailableRelease struct {
	Version     string `json:"version"`
	Name        string `json:"name,omitempty"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"publishedAt,omitempty"`
	URL         string `json:"url,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	AssetName   string `json:"assetName,omitempty"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt string    `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ListAvailable fetches GitHub releases and filters by the active channel.
// stable = non-prerelease only; pre = all non-draft releases.
func (m *Manager) ListAvailable(ctx context.Context) ([]AvailableRelease, error) {
	channel, err := m.Channel()
	if err != nil {
		return nil, err
	}
	raw, err := m.fetchGitHubReleases(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AvailableRelease, 0, len(raw))
	for _, rel := range raw {
		if rel.Draft {
			continue
		}
		if channel == ChannelStable && rel.Prerelease {
			continue
		}
		item := AvailableRelease{
			Version:     strings.TrimSpace(rel.TagName),
			Name:        strings.TrimSpace(rel.Name),
			Prerelease:  rel.Prerelease,
			PublishedAt: rel.PublishedAt,
		}
		if item.Version == "" {
			continue
		}
		asset, sumURL := pickReleaseAsset(rel.Assets, m.assetPattern)
		if asset != nil {
			item.URL = asset.BrowserDownloadURL
			item.AssetName = asset.Name
			if sum, err := m.resolveAssetSHA256(ctx, asset.Name, sumURL, rel.Body); err == nil {
				item.SHA256 = sum
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// ResolveManifest looks up a version in the available list (or returns man if complete).
func (m *Manager) ResolveManifest(ctx context.Context, man Manifest) (Manifest, error) {
	man.Version = strings.TrimSpace(man.Version)
	man.URL = strings.TrimSpace(man.URL)
	man.SHA256 = strings.TrimSpace(man.SHA256)
	if man.Version != "" && man.URL != "" && man.SHA256 != "" {
		return man, nil
	}
	if man.Version == "" {
		return Manifest{}, fmt.Errorf("version is required")
	}
	list, err := m.ListAvailable(ctx)
	if err != nil {
		return Manifest{}, err
	}
	want := man.Version
	wantTrim := strings.TrimPrefix(want, "v")
	for _, item := range list {
		tag := item.Version
		if tag != want && strings.TrimPrefix(tag, "v") != wantTrim {
			continue
		}
		if item.URL == "" {
			return Manifest{}, fmt.Errorf("release %s has no matching asset (%s)", tag, m.assetPattern)
		}
		if item.SHA256 == "" && man.SHA256 == "" {
			return Manifest{}, fmt.Errorf("release %s has no sha256 (CORE.SHA256 / body)", tag)
		}
		out := Manifest{
			Version:   tag,
			URL:       item.URL,
			SHA256:    item.SHA256,
			Changelog: man.Changelog,
			Channel:   man.Channel,
		}
		if man.SHA256 != "" {
			out.SHA256 = man.SHA256
		}
		if man.URL != "" {
			out.URL = man.URL
		}
		return out, nil
	}
	return Manifest{}, fmt.Errorf("version %q not found on channel %s", want, m.channelOrDefault())
}

func (m *Manager) fetchGitHubReleases(ctx context.Context) ([]ghRelease, error) {
	repo := strings.TrimSpace(m.githubRepo)
	if repo == "" {
		repo = "krate-apps/fleet"
	}
	if strings.Count(repo, "/") != 1 {
		return nil, fmt.Errorf("invalid github_repo %q (want owner/name)", repo)
	}
	base := strings.TrimRight(m.apiBase, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	url := base + "/repos/" + repo + "/releases?per_page=30"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fleet-updates")
	res, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("github releases: HTTP %d", res.StatusCode)
	}
	var list []ghRelease
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("github releases json: %w", err)
	}
	return list, nil
}

func pickReleaseAsset(assets []ghAsset, pattern string) (pkg *ghAsset, checksumURL string) {
	if pattern == "" {
		pattern = "fleet-*-linux-amd64.tgz"
	}
	var sum *ghAsset
	for i := range assets {
		a := &assets[i]
		name := a.Name
		if strings.EqualFold(name, "CORE.SHA256") || strings.EqualFold(name, "SHA256SUMS") ||
			strings.HasSuffix(strings.ToLower(name), ".sha256") {
			sum = a
		}
		ok, err := filepath.Match(pattern, name)
		if err == nil && ok {
			pkg = a
		}
	}
	if sum != nil {
		checksumURL = sum.BrowserDownloadURL
	}
	return pkg, checksumURL
}

func (m *Manager) resolveAssetSHA256(ctx context.Context, assetName, sumURL, body string) (string, error) {
	if sumURL != "" {
		if sum, err := m.downloadChecksum(ctx, sumURL, assetName); err == nil && sum != "" {
			return sum, nil
		}
	}
	if sum := parseChecksumText(body, assetName); sum != "" {
		return sum, nil
	}
	return "", fmt.Errorf("sha256 not found for %s", assetName)
}

func (m *Manager) downloadChecksum(ctx context.Context, rawURL, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fleet-updates")
	res, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("checksum HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	sum := parseChecksumText(string(raw), assetName)
	if sum == "" {
		return "", fmt.Errorf("no checksum line for %s", assetName)
	}
	return sum, nil
}

// parseChecksumText accepts sha256sum lines or a bare 64-hex digest.
func parseChecksumText(text, assetName string) string {
	want := strings.TrimSpace(assetName)
	baseWant := filepath.Base(want)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 1 && isSHA256Hex(fields[0]) {
			return strings.ToLower(fields[0])
		}
		if len(fields) >= 2 && isSHA256Hex(fields[0]) {
			name := strings.TrimPrefix(fields[1], "*")
			if name == want || filepath.Base(name) == baseWant || strings.Contains(name, baseWant) {
				return strings.ToLower(fields[0])
			}
		}
	}
	return ""
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
