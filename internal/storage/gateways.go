package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/krate-apps/fleet/internal/gateways"
)

// GatewayClient is a persisted remote BitTorrent WebUI/API endpoint.
type GatewayClient struct {
	ID          string        `json:"id"`
	Kind        gateways.Kind `json:"kind"`
	Name        string        `json:"name"`
	URL         string        `json:"url"`
	Username    string        `json:"username"`
	Password    string        `json:"-"`
	HasPassword bool          `json:"hasPassword"`
	Enabled     bool          `json:"enabled"`
	IsDefault   bool          `json:"isDefault"`
	JournalUnit string        `json:"journalUnit,omitempty"`
	LogPath     string        `json:"logPath,omitempty"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
}

const gatewaySelectCols = `id, kind, name, url, username, password, enabled, is_default, journal_unit, log_path, created_at, updated_at`

// ListGatewayClients returns all registered gateway clients (password omitted).
func (s *Store) ListGatewayClients() ([]GatewayClient, error) {
	return s.listGatewayClients(false)
}

// ListGatewayClientsWithSecrets returns all clients including passwords (sync poller).
func (s *Store) ListGatewayClientsWithSecrets() ([]GatewayClient, error) {
	return s.listGatewayClients(true)
}

func (s *Store) listGatewayClients(withSecrets bool) ([]GatewayClient, error) {
	rows, err := s.db.Query(`
SELECT ` + gatewaySelectCols + `
FROM gateway_clients
ORDER BY is_default DESC, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("list gateway clients: %w", err)
	}
	defer rows.Close()
	out := make([]GatewayClient, 0)
	for rows.Next() {
		c, err := scanGatewayClient(rows, withSecrets)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type gatewayScanner interface {
	Scan(dest ...any) error
}

func scanGatewayClient(row gatewayScanner, withSecrets bool) (GatewayClient, error) {
	var c GatewayClient
	var enabled, isDefault int
	var password string
	if err := row.Scan(
		&c.ID, &c.Kind, &c.Name, &c.URL, &c.Username, &password,
		&enabled, &isDefault, &c.JournalUnit, &c.LogPath, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return GatewayClient{}, err
	}
	c.Enabled = enabled != 0
	c.IsDefault = isDefault != 0
	c.HasPassword = password != ""
	if withSecrets {
		c.Password = password
	}
	return c, nil
}

// GetGatewayClient returns one client including password (for probe).
func (s *Store) GetGatewayClient(id string) (GatewayClient, error) {
	row := s.db.QueryRow(`SELECT `+gatewaySelectCols+` FROM gateway_clients WHERE id = ?`, id)
	c, err := scanGatewayClient(row, true)
	if err == sql.ErrNoRows {
		return GatewayClient{}, fmt.Errorf("gateway client not found")
	}
	if err != nil {
		return GatewayClient{}, err
	}
	return c, nil
}

// GetDefaultGatewayClient returns the configured default intake client, if any.
func (s *Store) GetDefaultGatewayClient() (GatewayClient, bool, error) {
	row := s.db.QueryRow(`SELECT ` + gatewaySelectCols + ` FROM gateway_clients WHERE is_default = 1 LIMIT 1`)
	c, err := scanGatewayClient(row, true)
	if err == sql.ErrNoRows {
		return GatewayClient{}, false, nil
	}
	if err != nil {
		return GatewayClient{}, false, err
	}
	c.IsDefault = true
	return c, true, nil
}

// SetDefaultGatewayClient marks id as the sole default intake client.
func (s *Store) SetDefaultGatewayClient(id string) error {
	if _, err := s.GetGatewayClient(id); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE gateway_clients SET is_default = 0`); err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE gateway_clients SET is_default = 1, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("gateway client not found")
	}
	return tx.Commit()
}

// ClearDefaultGatewayClient removes the default marker from all clients.
func (s *Store) ClearDefaultGatewayClient() error {
	_, err := s.db.Exec(`UPDATE gateway_clients SET is_default = 0`)
	return err
}

// UpsertGatewayClient inserts or updates a gateway client.
// If password is empty on update, the existing password is preserved when keepPassword is true.
func (s *Store) UpsertGatewayClient(c GatewayClient, keepPassword bool) (GatewayClient, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	enabled := 0
	if c.Enabled {
		enabled = 1
	}
	isDefault := 0
	if keepPassword {
		existing, err := s.GetGatewayClient(c.ID)
		if err == nil {
			if c.Password == "" {
				c.Password = existing.Password
			}
			if existing.IsDefault {
				isDefault = 1
				c.IsDefault = true
			}
		}
	} else if c.IsDefault {
		isDefault = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return GatewayClient{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if isDefault == 1 {
		if _, err := tx.Exec(`UPDATE gateway_clients SET is_default = 0`); err != nil {
			return GatewayClient{}, err
		}
	}
	_, err = tx.Exec(`
INSERT INTO gateway_clients(id, kind, name, url, username, password, enabled, is_default, journal_unit, log_path, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  kind=excluded.kind,
  name=excluded.name,
  url=excluded.url,
  username=excluded.username,
  password=excluded.password,
  enabled=excluded.enabled,
  is_default=excluded.is_default,
  journal_unit=excluded.journal_unit,
  log_path=excluded.log_path,
  updated_at=excluded.updated_at
`, c.ID, string(c.Kind), c.Name, c.URL, c.Username, c.Password, enabled, isDefault, c.JournalUnit, c.LogPath, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return GatewayClient{}, fmt.Errorf("upsert gateway client: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return GatewayClient{}, err
	}
	c.HasPassword = c.Password != ""
	c.Password = ""
	c.IsDefault = isDefault == 1
	return c, nil
}

// DeleteGatewayClient removes a gateway client.
func (s *Store) DeleteGatewayClient(id string) error {
	res, err := s.db.Exec(`DELETE FROM gateway_clients WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("gateway client not found")
	}
	return nil
}
