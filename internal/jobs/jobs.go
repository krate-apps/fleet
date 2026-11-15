package jobs

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/storage"
)

// Status values for jobs.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Job is a long-running unit of work for the core or an extension.
type Job struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Status      string          `json:"status"`
	ExtensionID string          `json:"extensionId,omitempty"`
	Progress    float64         `json:"progress"`
	Message     string          `json:"message,omitempty"`
	Error       string          `json:"error,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
	FinishedAt  string          `json:"finishedAt,omitempty"`
}

// CreateInput seeds a new job.
type CreateInput struct {
	Kind        string
	ExtensionID string
	Payload     any
}

// Manager persists and updates jobs in Fleet SQLite.
type Manager struct {
	store *storage.Store
}

// NewManager constructs a jobs manager.
func NewManager(store *storage.Store) *Manager {
	return &Manager{store: store}
}

// Create inserts a queued job and returns it.
func (m *Manager) Create(in CreateInput) (*Job, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("jobs manager unavailable")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		return nil, fmt.Errorf("kind required")
	}
	payload := []byte("{}")
	if in.Payload != nil {
		b, err := json.Marshal(in.Payload)
		if err != nil {
			return nil, fmt.Errorf("payload: %w", err)
		}
		payload = b
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id, err := newID()
	if err != nil {
		return nil, err
	}
	j := &Job{
		ID:          id,
		Kind:        kind,
		Status:      StatusQueued,
		ExtensionID: strings.TrimSpace(in.ExtensionID),
		Progress:    0,
		Payload:     payload,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = m.store.DB().Exec(`
INSERT INTO jobs(id, kind, status, extension_id, progress, message, error, payload_json, result_json, created_at, updated_at, finished_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.Kind, j.Status, j.ExtensionID, j.Progress, "", "", string(j.Payload), "", j.CreatedAt, j.UpdatedAt, "",
	)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}
	return j, nil
}

// Get returns a job by id.
func (m *Manager) Get(id string) (*Job, error) {
	row := m.store.DB().QueryRow(`
SELECT id, kind, status, extension_id, progress, message, error, payload_json, result_json, created_at, updated_at, finished_at
FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

// List returns recent jobs, optionally filtered by status.
func (m *Manager) List(status string, limit int) ([]Job, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if status != "" {
		rows, err = m.store.DB().Query(`
SELECT id, kind, status, extension_id, progress, message, error, payload_json, result_json, created_at, updated_at, finished_at
FROM jobs WHERE status = ? ORDER BY created_at DESC LIMIT ?`, status, limit)
	} else {
		rows, err = m.store.DB().Query(`
SELECT id, kind, status, extension_id, progress, message, error, payload_json, result_json, created_at, updated_at, finished_at
FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Job, 0)
	for rows.Next() {
		j, err := scanJobRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// Cancel marks a queued or running job as cancelled.
func (m *Manager) Cancel(id string) (*Job, error) {
	j, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	if j.Status != StatusQueued && j.Status != StatusRunning {
		return nil, fmt.Errorf("job %s is %s and cannot be cancelled", id, j.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = m.store.DB().Exec(`
UPDATE jobs SET status = ?, message = ?, updated_at = ?, finished_at = ? WHERE id = ?`,
		StatusCancelled, "cancelled", now, now, id,
	)
	if err != nil {
		return nil, err
	}
	return m.Get(id)
}

// MarkRunning transitions a job to running.
func (m *Manager) MarkRunning(id, message string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := m.store.DB().Exec(`
UPDATE jobs SET status = ?, message = ?, updated_at = ? WHERE id = ? AND status IN (?, ?)`,
		StatusRunning, message, now, id, StatusQueued, StatusRunning,
	)
	return err
}

// UpdateProgress sets progress (0..1) and message for a non-terminal job.
func (m *Manager) UpdateProgress(id string, progress float64, message string) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := m.store.DB().Exec(`
UPDATE jobs SET progress = ?, message = ?, status = ?, updated_at = ?
WHERE id = ? AND status IN (?, ?)`,
		progress, message, StatusRunning, now, id, StatusQueued, StatusRunning,
	)
	return err
}

// Succeed marks a job completed with an optional result payload.
func (m *Manager) Succeed(id string, result any) error {
	var resultJSON string
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		resultJSON = string(b)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := m.store.DB().Exec(`
UPDATE jobs SET status = ?, progress = 1, result_json = ?, error = '', updated_at = ?, finished_at = ? WHERE id = ?`,
		StatusSucceeded, resultJSON, now, now, id,
	)
	return err
}

// Fail marks a job failed.
func (m *Manager) Fail(id, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := m.store.DB().Exec(`
UPDATE jobs SET status = ?, error = ?, updated_at = ?, finished_at = ? WHERE id = ?`,
		StatusFailed, errMsg, now, now, id,
	)
	return err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanJob(row scannable) (*Job, error) {
	var j Job
	var payload, result string
	err := row.Scan(
		&j.ID, &j.Kind, &j.Status, &j.ExtensionID, &j.Progress, &j.Message, &j.Error,
		&payload, &result, &j.CreatedAt, &j.UpdatedAt, &j.FinishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, err
	}
	j.Payload = json.RawMessage(payload)
	if result != "" {
		j.Result = json.RawMessage(result)
	}
	return &j, nil
}

func scanJobRows(rows *sql.Rows) (*Job, error) {
	return scanJob(rows)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
