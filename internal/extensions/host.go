package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/sdk/backend"
	"github.com/krate-apps/fleet/sdk/manifest"
	"github.com/krate-apps/fleet/sdk/ui"
)

// Extension states persisted in SQLite.
const (
	StateDiscovered = "discovered"
	StateInactive   = "inactive"
	StateActive     = "active"
	StateError      = "error"
)

// Record is the host view of an installed/discovered extension.
type Record struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	Version         string             `json:"version"`
	Path            string             `json:"path"`
	State           string             `json:"state"`
	Error           string             `json:"error,omitempty"`
	Manifest        *manifest.Manifest `json:"manifest"`
	UI              *ui.Contribution   `json:"ui,omitempty"`
	UpdatedAt       string             `json:"updatedAt"`
	Official        bool               `json:"official"`
	ConsentRequired bool               `json:"consentRequired"`
	Consented       bool               `json:"consented"`
	ConsentWarning  string             `json:"consentWarning,omitempty"`
}

// Host discovers extension packages on disk and tracks activation state.
// Backend workers are supervised as external processes (JSON-RPC); crashes stay isolated.
type Host struct {
	mu        sync.RWMutex
	store     *storage.Store
	roots     []string
	records   map[string]*Record
	workers   map[string]*WorkerSession
	ctx       context.Context
	dataDir   string
	bridge    HostBridge
	workerEnv map[string]string
}

// HostOptions configures discovery roots and worker host services.
type HostOptions struct {
	Roots    []string
	DataDir  string
	Bridge   HostBridge
	WorkerEnv map[string]string // extra env for workers (e.g. FLEET_TOOL_RCLONE)
}

// NewHost creates an extension host scanning the given package roots.
func NewHost(store *storage.Store, roots ...string) *Host {
	return NewHostWithOptions(context.Background(), store, HostOptions{Roots: roots})
}

// NewHostContext is like NewHost but ties worker lifetimes to ctx.
func NewHostContext(ctx context.Context, store *storage.Store, roots ...string) *Host {
	return NewHostWithOptions(ctx, store, HostOptions{Roots: roots})
}

// NewHostWithOptions constructs a fully configured extension host.
func NewHostWithOptions(ctx context.Context, store *storage.Store, opts HostOptions) *Host {
	if ctx == nil {
		ctx = context.Background()
	}
	clean := make([]string, 0, len(opts.Roots))
	for _, r := range opts.Roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		clean = append(clean, r)
	}
	h := &Host{
		store:   store,
		roots:   clean,
		records: map[string]*Record{},
		workers: map[string]*WorkerSession{},
		ctx:     ctx,
		dataDir: opts.DataDir,
		bridge:  opts.Bridge,
	}
	if len(opts.WorkerEnv) > 0 {
		h.workerEnv = map[string]string{}
		for k, v := range opts.WorkerEnv {
			h.workerEnv[k] = v
		}
	}
	return h
}

// Refresh scans roots for fleet.ext.yaml / extension.yaml packages and syncs SQLite.
func (h *Host) Refresh() error {
	if h == nil {
		return fmt.Errorf("extension host unavailable")
	}
	found := map[string]*Record{}
	for _, root := range h.roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read extensions root %s: %w", root, err)
		}
		for _, ent := range entries {
			if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			dir := filepath.Join(root, ent.Name())
			// Skip dirs with no manifest at all (e.g. runtime data folders mistyped as a
			// discovery root). Broken manifests still surface as invalid.* records.
			if _, err := findManifest(dir); err != nil {
				continue
			}
			rec, err := loadPackage(dir)
			if err != nil {
				// Keep scanning; surface broken packages as error records keyed by folder name.
				now := time.Now().UTC().Format(time.RFC3339Nano)
				id := "invalid." + ent.Name()
				found[id] = &Record{
					ID:        id,
					Name:      ent.Name(),
					Version:   "0.0.0",
					Path:      dir,
					State:     StateError,
					Error:     err.Error(),
					UpdatedAt: now,
				}
				continue
			}
			found[rec.ID] = rec
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Preserve activation state from SQLite when present.
	for id, rec := range found {
		state, errMsg, ok := h.loadState(id)
		if ok {
			if state == StateActive || state == StateInactive {
				rec.State = state
			}
			if errMsg != "" && rec.Error == "" {
				rec.Error = errMsg
			}
		}
		h.annotateConsent(rec)
		if err := h.upsert(rec); err != nil {
			return err
		}
		h.records[id] = rec
	}
	// Drop records that disappeared from disk (keep DB row as inactive tombstone optional — remove for now).
	for id := range h.records {
		if _, ok := found[id]; !ok {
			delete(h.records, id)
		}
	}
	return nil
}

// List returns discovered extensions.
func (h *Host) List() []Record {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Record, 0, len(h.records))
	for _, rec := range h.records {
		h.annotateConsent(rec)
		out = append(out, *rec)
	}
	return out
}

// Get returns one extension by id.
func (h *Host) Get(id string) (*Record, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	rec, ok := h.records[id]
	if !ok {
		return nil, fmt.Errorf("extension not found")
	}
	h.annotateConsent(rec)
	cp := *rec
	return &cp, nil
}

// Activate marks an extension active after manifest re-validation and starts its worker when present.
func (h *Host) Activate(id string) (*Record, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec, ok := h.records[id]
	if !ok {
		return nil, fmt.Errorf("extension not found")
	}
	if h.ConsentRequired(id) {
		ok, err := h.HasConsent(id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("consent required: %s", ConsentWarning())
		}
	}
	if rec.State == StateError || rec.Manifest == nil {
		return nil, fmt.Errorf("extension %s cannot be activated: %s", id, rec.Error)
	}
	if err := rec.Manifest.Validate(); err != nil {
		rec.State = StateError
		rec.Error = err.Error()
		_ = h.upsert(rec)
		return nil, err
	}
	backendWarning := ""
	if rec.Manifest.Backend != nil {
		if existing := h.workers[id]; existing != nil {
			_ = existing.Close()
			delete(h.workers, id)
		}
		sess, err := StartWorker(h.ctx, rec.Manifest, rec.Path, WorkerStartOptions{
			DataDir: DataDir(h.dataDir, rec.ID),
			Bridge:  h.bridge,
			Env:     h.workerEnv,
		})
		if err != nil {
			// UI-only activate when the worker binary is not built yet but frontend exists.
			if rec.Manifest.Frontend != nil && isMissingWorker(err) {
				backendWarning = err.Error()
			} else {
				rec.State = StateError
				rec.Error = err.Error()
				rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				_ = h.upsert(rec)
				return nil, err
			}
		} else {
			if _, err := sess.Call(h.ctx, backend.MethodActivate, map[string]any{}); err != nil {
				_ = sess.Close()
				rec.State = StateError
				rec.Error = err.Error()
				rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				_ = h.upsert(rec)
				return nil, fmt.Errorf("ext.activate: %w", err)
			}
			h.workers[id] = sess
			go h.watchWorker(id, sess)
		}
	}
	rec.State = StateActive
	rec.Error = backendWarning
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := h.upsert(rec); err != nil {
		return nil, err
	}
	h.annotateConsent(rec)
	cp := *rec
	return &cp, nil
}

// Deactivate stops the worker (if any) and marks an extension inactive.
func (h *Host) Deactivate(id string) (*Record, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec, ok := h.records[id]
	if !ok {
		return nil, fmt.Errorf("extension not found")
	}
	if sess := h.workers[id]; sess != nil {
		_, _ = sess.Call(h.ctx, backend.MethodDeactivate, map[string]any{})
		_ = sess.Close()
		delete(h.workers, id)
	}
	rec.State = StateInactive
	rec.Error = ""
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := h.upsert(rec); err != nil {
		return nil, err
	}
	cp := *rec
	return &cp, nil
}

// watchWorker isolates worker crashes from the Fleet process.
func (h *Host) watchWorker(id string, sess *WorkerSession) {
	<-sess.done
	h.mu.Lock()
	defer h.mu.Unlock()
	current, ok := h.workers[id]
	if !ok || current != sess {
		return
	}
	delete(h.workers, id)
	rec, ok := h.records[id]
	if !ok {
		return
	}
	if rec.State != StateActive {
		return
	}
	msg := "worker exited"
	if sess.err != nil {
		msg = sess.err.Error()
	}
	rec.State = StateError
	rec.Error = msg
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = h.upsert(rec)
}

// DataDir returns the per-extension runtime data directory under Fleet's data root.
// Kept separate from package discovery roots (extensions-src / extensions_dir) so
// activate/storage MkdirAll cannot create invalid.* ghosts in a scanned tree.
func DataDir(fleetDataDir, extensionID string) string {
	safe := strings.ReplaceAll(extensionID, "/", "_")
	base := fleetDataDir
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "extension-data", safe)
}

// CallWorker proxies a JSON-RPC method to an active worker.
func (h *Host) CallWorker(ctx context.Context, id, method string, params any) (json.RawMessage, error) {
	h.mu.RLock()
	sess := h.workers[id]
	h.mu.RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("extension worker not active")
	}
	return sess.Call(ctx, method, params)
}

// Storage returns a storage helper bound to this host data dir.
func (h *Host) Storage() *StorageBridge {
	return &StorageBridge{DataDir: h.dataDir}
}

func loadPackage(dir string) (*Record, error) {
	manifestPath, err := findManifest(dir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	m, err := manifest.ParseYAML(raw)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec := &Record{
		ID:          m.ID,
		Name:        m.Name,
		Description: strings.TrimSpace(m.Description),
		Version:     m.Version,
		Path:        dir,
		State:       StateDiscovered,
		Manifest:    m,
		UpdatedAt:   now,
	}
	if m.Frontend != nil {
		uiPath := filepath.Join(dir, filepath.FromSlash(m.Frontend.Entry))
		uiRaw, err := os.ReadFile(uiPath)
		if err != nil {
			return nil, fmt.Errorf("read ui entry: %w", err)
		}
		contrib, err := ui.ParseJSON(uiRaw)
		if err != nil {
			return nil, err
		}
		if contrib.ExtensionID != m.ID {
			return nil, fmt.Errorf("ui extensionId %q does not match manifest id %q", contrib.ExtensionID, m.ID)
		}
		rec.UI = contrib
	}
	return rec, nil
}

func isMissingWorker(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "worker executable") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "not runnable")
}

func findManifest(dir string) (string, error) {
	for _, name := range []string{"fleet.ext.yaml", "extension.yaml", "manifest.yaml"} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no fleet.ext.yaml found in %s", dir)
}

func (h *Host) upsert(rec *Record) error {
	if h.store == nil {
		return nil
	}
	manifestJSON := "{}"
	if rec.Manifest != nil {
		b, err := json.Marshal(rec.Manifest)
		if err != nil {
			return err
		}
		manifestJSON = string(b)
	}
	now := rec.UpdatedAt
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := h.store.DB().Exec(`
INSERT INTO extensions(id, name, version, path, state, manifest_json, error, installed_at, updated_at)
VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
  name=excluded.name,
  version=excluded.version,
  path=excluded.path,
  state=excluded.state,
  manifest_json=excluded.manifest_json,
  error=excluded.error,
  updated_at=excluded.updated_at
`, rec.ID, rec.Name, rec.Version, rec.Path, rec.State, manifestJSON, rec.Error, now, now)
	return err
}

func (h *Host) loadState(id string) (state, errMsg string, ok bool) {
	if h.store == nil {
		return "", "", false
	}
	err := h.store.DB().QueryRow(
		`SELECT state, error FROM extensions WHERE id = ?`, id,
	).Scan(&state, &errMsg)
	if err != nil {
		return "", "", false
	}
	return state, errMsg, true
}
