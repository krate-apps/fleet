package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/buckets"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/version"
)

// Status describes the current instance and any pending update (UPD-001).
type Status struct {
	CurrentVersion string          `json:"currentVersion"`
	CurrentCommit  string          `json:"currentCommit"`
	Channel        string          `json:"channel"`
	Pending        *PendingRelease `json:"pending,omitempty"`
	LastBackup     string          `json:"lastBackup,omitempty"`
	AutoUpdate     bool            `json:"autoUpdate"` // always false in V1
	InstallPrefix  string          `json:"installPrefix,omitempty"`
	GitHubRepo     string          `json:"githubRepo,omitempty"`
}

// PendingRelease is a downloaded, validated package waiting for explicit apply.
type PendingRelease struct {
	Version    string `json:"version"`
	URL        string `json:"url"`
	SHA256     string `json:"sha256"`
	Changelog  string `json:"changelog,omitempty"`
	Downloaded string `json:"downloadedAt"`
	Path       string `json:"path"`
}

// Manifest is the signed-by-checksum update descriptor (no auto-apply).
type Manifest struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Changelog string `json:"changelog,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

// ManagerConfig wires optional install prefix and GitHub discovery.
type ManagerConfig struct {
	InstallPrefix string
	GitHubRepo    string
	AssetPattern  string
	// APIBase overrides https://api.github.com (tests).
	APIBase string
}

// Manager prepares core updates without applying them automatically (UPD-001/002).
type Manager struct {
	dataDir       string
	releasesDir   string
	installPrefix string
	githubRepo    string
	assetPattern  string
	apiBase       string
	store         *storage.Store
	httpClient    *http.Client
	channel       string
}

// NewManager constructs an update manager rooted under the instance data dir.
func NewManager(store *storage.Store, dataDir string, cfg ...ManagerConfig) *Manager {
	var c ManagerConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	repo := strings.TrimSpace(c.GitHubRepo)
	if repo == "" {
		repo = "krate-apps/fleet"
	}
	pattern := strings.TrimSpace(c.AssetPattern)
	if pattern == "" {
		pattern = "fleet-*-linux-amd64.tgz"
	}
	m := &Manager{
		dataDir:       dataDir,
		releasesDir:   filepath.Join(dataDir, "updates"),
		installPrefix: strings.TrimSpace(c.InstallPrefix),
		githubRepo:    repo,
		assetPattern:  pattern,
		apiBase:       strings.TrimSpace(c.APIBase),
		store:         store,
		httpClient:    &http.Client{Timeout: 2 * time.Minute},
		channel:       ChannelStable,
	}
	if ch, err := m.Channel(); err == nil {
		m.channel = ch
	}
	return m
}

// Status returns current version info and any pending package.
func (m *Manager) Status() (*Status, error) {
	ch, _ := m.Channel()
	st := &Status{
		CurrentVersion: version.Version,
		CurrentCommit:  version.Commit,
		Channel:        ch,
		AutoUpdate:     false,
		InstallPrefix:  m.installPrefix,
		GitHubRepo:     m.githubRepo,
	}
	pendingPath := filepath.Join(m.releasesDir, "pending.json")
	if raw, err := os.ReadFile(pendingPath); err == nil {
		var p PendingRelease
		if json.Unmarshal(raw, &p) == nil {
			st.Pending = &p
		}
	}
	var last string
	_ = m.store.DB().QueryRow(`SELECT value FROM meta WHERE key = 'last_backup'`).Scan(&last)
	st.LastBackup = last
	return st, nil
}

// Prepare downloads and verifies a release archive without activating it (UPD-002).
// Version-only manifests are resolved via GitHub available releases.
func (m *Manager) Prepare(ctx context.Context, man Manifest) (*PendingRelease, error) {
	resolved, err := m.ResolveManifest(ctx, man)
	if err != nil {
		return nil, err
	}
	man = resolved
	if err := os.MkdirAll(m.releasesDir, 0o700); err != nil {
		return nil, err
	}
	archive := filepath.Join(m.releasesDir, "pending-"+sanitize(man.Version)+".tar.gz")
	if err := downloadFile(ctx, m.httpClient, man.URL, archive); err != nil {
		return nil, err
	}
	if err := buckets.VerifySHA256(archive, man.SHA256); err != nil {
		_ = os.Remove(archive)
		return nil, err
	}
	extractDir := filepath.Join(m.releasesDir, "pending-"+sanitize(man.Version))
	_ = os.RemoveAll(extractDir)
	if err := buckets.ExtractTarGz(archive, extractDir); err != nil {
		_ = os.Remove(archive)
		return nil, err
	}
	pending := &PendingRelease{
		Version:    man.Version,
		URL:        man.URL,
		SHA256:     strings.ToLower(strings.TrimSpace(man.SHA256)),
		Changelog:  man.Changelog,
		Downloaded: time.Now().UTC().Format(time.RFC3339Nano),
		Path:       extractDir,
	}
	raw, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(m.releasesDir, "pending.json"), raw, 0o600); err != nil {
		return nil, err
	}
	return pending, nil
}

// BackupNow creates a coherent SQLite snapshot before apply/rollback (UPD-004).
func (m *Manager) BackupNow(ctx context.Context) (string, error) {
	path := storage.NewBackupPath(m.dataDir)
	if err := m.store.BackupSQLite(ctx, path); err != nil {
		return "", err
	}
	_, _ = m.store.DB().Exec(
		`INSERT INTO meta(key, value) VALUES('last_backup', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, path,
	)
	return path, nil
}

// ClearPending removes a prepared update without activating it.
func (m *Manager) ClearPending() error {
	var p PendingRelease
	raw, err := os.ReadFile(filepath.Join(m.releasesDir, "pending.json"))
	if err == nil && json.Unmarshal(raw, &p) == nil && p.Path != "" {
		_ = os.RemoveAll(p.Path)
	}
	_ = os.Remove(filepath.Join(m.releasesDir, "pending.json"))
	return nil
}

// ApplyPlan describes what an explicit apply would do. Flipping <prefix>/current
// stays outside the HTTP process (OPEN-08 / UPD-003 / SEC-009).
type ApplyPlan struct {
	BackupPath     string `json:"backupPath"`
	PendingPath    string `json:"pendingPath,omitempty"`
	PendingVersion string `json:"pendingVersion"`
	InstalledPath  string `json:"installedPath,omitempty"`
	HelperCommand  string `json:"helperCommand"`
	ApplyRequest   string `json:"applyRequestPath,omitempty"`
	Copied         bool   `json:"copied,omitempty"`
	Message        string `json:"message"`
}

// PlanApplyOptions configures helper hints written with the apply plan.
type PlanApplyOptions struct {
	InstallPrefix string
	HealthURL     string
}

// PlanApply backs up SQLite and returns helper instructions; it never auto-restarts (UPD-001).
func (m *Manager) PlanApply(ctx context.Context) (*ApplyPlan, error) {
	return m.PlanApplyWithOptions(ctx, PlanApplyOptions{})
}

// PlanApplyWithOptions is PlanApply plus install-prefix / health URL for the helper.
// When the prefix is writable, the pending fleet binary is copied into releases/<ver>/.
func (m *Manager) PlanApplyWithOptions(ctx context.Context, opts PlanApplyOptions) (*ApplyPlan, error) {
	st, err := m.Status()
	if err != nil {
		return nil, err
	}
	if st.Pending == nil {
		return nil, fmt.Errorf("no pending update to apply")
	}
	backup, err := m.BackupNow(ctx)
	if err != nil {
		return nil, err
	}
	prefix := strings.TrimSpace(opts.InstallPrefix)
	if prefix == "" {
		prefix = m.installPrefix
	}
	health := strings.TrimSpace(opts.HealthURL)
	if health == "" {
		health = "http://127.0.0.1:8080/api/v1/health"
	}
	helper := HelperCommand(prefix, st.Pending.Version, health)
	applyPath := ""
	installed := ""
	copied := false
	msg := "Backup complete. Copy the pending tree into <prefix>/releases/<version>, then point <prefix>/current at that release and restart Fleet (UPD-003)."
	if prefix != "" {
		if dest, copyErr := m.InstallPendingToPrefix(prefix); copyErr == nil {
			installed = dest
			copied = true
			msg = "Backup complete. Pending binary copied into releases; point <prefix>/current at the new release and restart Fleet (UPD-003)."
		} else if !os.IsPermission(copyErr) && !isNotExistOrNotWritable(copyErr) {
			// keep going with helper-only path; surface copy failure in message
			msg = fmt.Sprintf("Backup complete. Could not copy into prefix (%v). Copy manually, then flip current and restart Fleet.", copyErr)
		}
		if req, err := m.WriteApplyRequest(prefix, health, backup); err == nil {
			helper = req.Helper
			applyPath = filepath.Join(m.releasesDir, "apply-request.json")
		}
	}
	return &ApplyPlan{
		BackupPath:     backup,
		PendingPath:    st.Pending.Path,
		PendingVersion: st.Pending.Version,
		InstalledPath:  installed,
		HelperCommand:  helper,
		ApplyRequest:   applyPath,
		Copied:         copied,
		Message:        msg,
	}, nil
}

// PlanSwitch plans a flip to an already-installed release under install_prefix.
func (m *Manager) PlanSwitch(ctx context.Context, version string, opts PlanApplyOptions) (*ApplyPlan, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version required")
	}
	prefix := strings.TrimSpace(opts.InstallPrefix)
	if prefix == "" {
		prefix = m.installPrefix
	}
	if prefix == "" {
		return nil, fmt.Errorf("install_prefix is not configured")
	}
	layout := Layout{Prefix: prefix}
	bin := filepath.Join(layout.ReleaseDir(version), "fleet")
	st, err := os.Stat(bin)
	if err != nil || st.IsDir() {
		return nil, fmt.Errorf("installed release %q not found under %s", version, prefix)
	}
	backup, err := m.BackupNow(ctx)
	if err != nil {
		return nil, err
	}
	health := strings.TrimSpace(opts.HealthURL)
	if health == "" {
		health = "http://127.0.0.1:8080/api/v1/health"
	}
	helper := HelperCommand(prefix, sanitize(version), health)
	req := &ApplyRequest{
		Prefix:    prefix,
		Version:   sanitize(version),
		HealthURL: health,
		Helper:    helper,
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
	applyPath := filepath.Join(m.releasesDir, "apply-request.json")
	if err := os.WriteFile(applyPath, raw, 0o600); err != nil {
		return nil, err
	}
	return &ApplyPlan{
		BackupPath:     backup,
		PendingVersion: sanitize(version),
		InstalledPath:  layout.ReleaseDir(version),
		HelperCommand:  helper,
		ApplyRequest:   applyPath,
		Message:        "Backup complete. Point <prefix>/current at the selected release and restart Fleet.",
	}, nil
}

// InstallPendingToPrefix copies the pending fleet binary into <prefix>/releases/<ver>/fleet.
func (m *Manager) InstallPendingToPrefix(prefix string) (string, error) {
	st, err := m.Status()
	if err != nil {
		return "", err
	}
	if st.Pending == nil {
		return "", fmt.Errorf("no pending update")
	}
	src, err := findFleetBinary(st.Pending.Path)
	if err != nil {
		return "", err
	}
	layout := Layout{Prefix: prefix}
	destDir := layout.ReleaseDir(st.Pending.Version)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, "fleet")
	if err := copyFile(src, dest, 0o755); err != nil {
		return "", err
	}
	return destDir, nil
}

func findFleetBinary(root string) (string, error) {
	candidates := []string{
		filepath.Join(root, "fleet"),
		filepath.Join(root, "bin", "fleet"),
		filepath.Join(root, "fleet-lab", "bin", "fleet"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || found != "" {
			return nil
		}
		if info.Name() == "fleet" && info.Mode().IsRegular() {
			found = path
			return io.EOF
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("fleet binary not found under pending path %s", root)
	}
	return found, nil
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func isNotExistOrNotWritable(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) || os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") || strings.Contains(msg, "read-only")
}

func sanitize(v string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "..", "_", " ", "_")
	return r.Replace(v)
}

func downloadFile(ctx context.Context, client *http.Client, rawURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("download failed: HTTP %d", res.StatusCode)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(res.Body, 512<<20))
	return err
}

// HashFile is a small helper for tests and tooling.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
