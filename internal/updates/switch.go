package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Layout describes a per-user Fleet install tree (INS-006 / UPD-005).
//
//	<prefix>/
//	  releases/<version>/fleet
//	  current -> releases/<version>
type Layout struct {
	Prefix string
}

// CurrentLink returns <prefix>/current.
func (l Layout) CurrentLink() string {
	return filepath.Join(l.Prefix, "current")
}

// ReleaseDir returns <prefix>/releases/<version>.
func (l Layout) ReleaseDir(version string) string {
	return filepath.Join(l.Prefix, "releases", sanitize(version))
}

// InstalledRelease is a version directory under <prefix>/releases.
type InstalledRelease struct {
	Version   string `json:"version"`
	Path      string `json:"path"`
	Current   bool   `json:"current"`
	HasBinary bool   `json:"hasBinary"`
}

// ListInstalled returns release dirs under the layout prefix.
func (l Layout) ListInstalled() ([]InstalledRelease, error) {
	if strings.TrimSpace(l.Prefix) == "" {
		return nil, fmt.Errorf("install prefix required")
	}
	root := filepath.Join(l.Prefix, "releases")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []InstalledRelease{}, nil
		}
		return nil, err
	}
	current, _ := l.ResolveCurrentVersion()
	out := make([]InstalledRelease, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name == "." || name == ".." {
			continue
		}
		bin := filepath.Join(root, name, "fleet")
		st, err := os.Stat(bin)
		has := err == nil && !st.IsDir()
		out = append(out, InstalledRelease{
			Version:   name,
			Path:      filepath.Join(root, name),
			Current:   name == current,
			HasBinary: has,
		})
	}
	return out, nil
}

// ListInstalled on Manager uses the configured install prefix.
func (m *Manager) ListInstalled() ([]InstalledRelease, error) {
	prefix := strings.TrimSpace(m.installPrefix)
	if prefix == "" {
		return []InstalledRelease{}, nil
	}
	return (Layout{Prefix: prefix}).ListInstalled()
}

// ResolveCurrentVersion reads the current symlink target basename.
func (l Layout) ResolveCurrentVersion() (string, error) {
	target, err := os.Readlink(l.CurrentLink())
	if err != nil {
		return "", err
	}
	target = filepath.Clean(target)
	base := filepath.Base(target)
	if base == "" || base == "." || base == ".." {
		return "", fmt.Errorf("invalid current symlink target %q", target)
	}
	return base, nil
}

// SwitchCurrent atomically points current at releases/<version> and returns the previous version (if any).
func (l Layout) SwitchCurrent(version string) (previous string, err error) {
	version = sanitize(version)
	rel := l.ReleaseDir(version)
	st, err := os.Stat(filepath.Join(rel, "fleet"))
	if err != nil {
		return "", fmt.Errorf("release binary missing: %w", err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("release binary is a directory")
	}
	prev, _ := l.ResolveCurrentVersion()

	link := l.CurrentLink()
	tmp := link + ".new"
	_ = os.Remove(tmp)
	// Relative symlink keeps the tree relocatable.
	if err := os.Symlink(filepath.Join("releases", version), tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return prev, nil
}

// HealthCheck polls GET url until success or timeout (UPD-003).
func HealthCheck(ctx context.Context, client *http.Client, url string, timeout time.Duration) error {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	deadline := time.Now().Add(timeout)
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK && strings.Contains(string(body), `"status"`) {
				return nil
			}
			last = fmt.Errorf("health HTTP %d", res.StatusCode)
		} else {
			last = err
		}
		if time.Now().After(deadline) {
			if last == nil {
				last = fmt.Errorf("health check timed out")
			}
			return last
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// SwitchOptions controls a promote + health + rollback cycle.
type SwitchOptions struct {
	Version    string
	HealthURL  string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// SwitchResult is the outcome of SwitchWithHealth.
type SwitchResult struct {
	Previous string `json:"previous,omitempty"`
	Current  string `json:"current"`
	RolledBack bool `json:"rolledBack"`
	Message  string `json:"message"`
}

// SwitchWithHealth flips current, checks health, and restores the previous release on failure (UPD-003).
func (l Layout) SwitchWithHealth(ctx context.Context, opts SwitchOptions) (*SwitchResult, error) {
	if strings.TrimSpace(opts.Version) == "" {
		return nil, fmt.Errorf("version required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	prev, err := l.SwitchCurrent(opts.Version)
	if err != nil {
		return nil, err
	}
	res := &SwitchResult{Previous: prev, Current: sanitize(opts.Version)}
	if strings.TrimSpace(opts.HealthURL) == "" {
		res.Message = "switched without health check"
		return res, nil
	}
	if err := HealthCheck(ctx, opts.HTTPClient, opts.HealthURL, opts.Timeout); err != nil {
		if prev != "" && prev != res.Current {
			if _, rbErr := l.SwitchCurrent(prev); rbErr != nil {
				return res, fmt.Errorf("health failed (%v) and rollback failed (%w)", err, rbErr)
			}
			res.RolledBack = true
			res.Current = prev
			res.Message = fmt.Sprintf("health failed after switch: %v; restored %s", err, prev)
			return res, fmt.Errorf("%s", res.Message)
		}
		res.Message = fmt.Sprintf("health failed after switch: %v (no previous release to restore)", err)
		return res, fmt.Errorf("%s", res.Message)
	}
	res.Message = "switch healthy"
	return res, nil
}

// HelperCommand documents how an operator flips current after PlanApply (UPD-003 / SEC-009).
// The HTTP process never rewrites the install symlink itself.
func HelperCommand(prefix, version, healthURL string) string {
	_ = healthURL
	prefix = strings.TrimSpace(prefix)
	version = strings.TrimSpace(version)
	if prefix == "" || version == "" {
		return "ln -sfn releases/<version> <prefix>/current && restart the Fleet service"
	}
	return fmt.Sprintf(
		"ln -sfn %s %s && restart the Fleet service (e.g. systemctl restart fleet@<user>)",
		shellQuote("releases/"+sanitize(version)),
		shellQuote(filepath.Join(prefix, "current")),
	)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// WriteApplyRequest stores a helper request next to pending.json for operators.
type ApplyRequest struct {
	Prefix    string `json:"prefix"`
	Version   string `json:"version"`
	HealthURL string `json:"healthUrl,omitempty"`
	Helper    string `json:"helper"`
	Backup    string `json:"backupPath,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// WriteApplyRequest persists the helper invocation hint under the updates dir.
func (m *Manager) WriteApplyRequest(prefix, healthURL, backup string) (*ApplyRequest, error) {
	st, err := m.Status()
	if err != nil {
		return nil, err
	}
	if st.Pending == nil {
		return nil, fmt.Errorf("no pending update")
	}
	req := &ApplyRequest{
		Prefix:    prefix,
		Version:   st.Pending.Version,
		HealthURL: healthURL,
		Helper:    HelperCommand(prefix, st.Pending.Version, healthURL),
		Backup:    backup,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(m.releasesDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(m.releasesDir, "apply-request.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	return req, nil
}
