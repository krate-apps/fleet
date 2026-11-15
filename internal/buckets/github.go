package buckets

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// githubReleaseAsset matches https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}
var githubReleaseAsset = regexp.MustCompile(`(?i)^https://github\.com/[^/]+/[^/]+/releases/download/[^/]+/[^/]+$`)

// RecognizeGitHubPackageURL validates a GitHub Releases asset URL (BKT-004).
// Returns a short diagnostic when the URL is not a pinned release download.
func RecognizeGitHubPackageURL(raw string) error {
	u, err := validatePackageURL(raw)
	if err != nil {
		return err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("GitHub package URLs must use https")
	}
	if !strings.EqualFold(u.Host, "github.com") && !strings.EqualFold(u.Host, "www.github.com") {
		return fmt.Errorf("not a github.com URL (got host %q)", u.Host)
	}
	if !githubReleaseAsset.MatchString(strings.TrimSuffix(u.String(), "/")) {
		return fmt.Errorf("expected GitHub Releases download URL (.../releases/download/<tag>/<asset>), got %q", raw)
	}
	if strings.Contains(u.Path, "/archive/") || strings.HasSuffix(u.Path, ".git") {
		return fmt.Errorf("branch/archive URLs are not allowed; pin a release asset")
	}
	return nil
}

// NormalizePackageURL trims and rejects empty URLs.
func NormalizePackageURL(raw string) (string, error) {
	u, err := validatePackageURL(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// IsGitHubReleaseURL reports whether raw looks like a GitHub release asset URL.
func IsGitHubReleaseURL(raw string) bool {
	return RecognizeGitHubPackageURL(raw) == nil
}

// DescribePackageURL returns a human-readable origin label for UI/diagnostics.
func DescribePackageURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "invalid"
	}
	if IsGitHubReleaseURL(raw) {
		return "github-release"
	}
	return u.Scheme + "://" + u.Host
}
