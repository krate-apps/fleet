// Package theme validates Fleet theme package manifests (fleet.theme.yaml).
package theme

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

// Manifest is fleet.theme.yaml.
type Manifest struct {
	Schema      int    `yaml:"schema" json:"schema"`
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string `yaml:"version" json:"version"`
	ColorScheme string `yaml:"colorScheme" json:"colorScheme"`
	CSS         string `yaml:"css" json:"css"`
	UIThemeID   string `yaml:"uiThemeId,omitempty" json:"uiThemeId,omitempty"`
}

// ParseYAML validates a theme package manifest.
func ParseYAML(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse theme: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks docs/developer/schemas/theme.schema.json constraints.
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("theme manifest is nil")
	}
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported theme schema %d", m.Schema)
	}
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("theme id required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("theme name required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("theme version required")
	}
	switch m.ColorScheme {
	case "dark", "light":
	default:
		return fmt.Errorf("colorScheme must be dark or light")
	}
	if strings.TrimSpace(m.CSS) == "" {
		return fmt.Errorf("css path required")
	}
	if strings.Contains(m.CSS, "..") || strings.HasPrefix(m.CSS, "/") {
		return fmt.Errorf("css path must be relative without ..")
	}
	if len(m.Description) > 280 {
		return fmt.Errorf("description must be <= 280 characters")
	}
	return nil
}

// ShortID returns the data-fleet-theme attribute value.
func (m *Manifest) ShortID() string {
	if s := strings.TrimSpace(m.UIThemeID); s != "" {
		return s
	}
	parts := strings.Split(m.ID, ".")
	if len(parts) == 0 {
		return m.ID
	}
	return parts[len(parts)-1]
}
