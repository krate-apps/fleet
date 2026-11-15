package buckets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/netutil"
)

const httpTimeout = 2 * time.Minute

// DownloadOptions controls remote package fetches (BKT-004).
type DownloadOptions struct {
	HTTPClient *http.Client
}

func (r *Registry) httpClient(opts DownloadOptions) *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return netutil.SafeHTTPClient(httpTimeout)
}

// validatePackageURL allows http(s) only and rejects private/link-local hosts.
func validatePackageURL(raw string) (*url.URL, error) {
	return netutil.ValidatePublicURL(raw)
}

func validatePackageURLScheme(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("only http(s) package URLs are allowed")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("package URL host required")
	}
	return u, nil
}

// FetchToFile downloads url into destPath with size limits.
// Production clients (nil HTTPClient) use SSRF-safe dialing; custom clients are for tests only.
func (r *Registry) FetchToFile(ctx context.Context, rawURL, destPath string, opts DownloadOptions) error {
	var (
		u   *url.URL
		err error
	)
	if opts.HTTPClient == nil {
		u, err = validatePackageURL(rawURL)
	} else {
		u, err = validatePackageURLScheme(rawURL)
	}
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Fleet-Buckets/1.0")
	res, err := r.httpClient(opts).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("download failed: HTTP %d", res.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(res.Body, maxArchiveBytes+1))
	if err != nil {
		return err
	}
	if n > maxArchiveBytes {
		_ = os.Remove(destPath)
		return fmt.Errorf("download exceeds %d bytes", maxArchiveBytes)
	}
	return nil
}
