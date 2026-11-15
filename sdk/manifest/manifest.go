// Package manifest defines and validates Fleet extension package manifests (OPEN-05).
package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-+][A-Za-z0-9.-]+)?$`)
)

// KnownEngines are engines an extension may declare.
var KnownEngines = map[string]struct{}{
	"rtorrent":      {},
	"qbittorrent":   {},
	"deluge":        {},
	"transmission":  {},
}

// KnownCapabilities are capabilities Fleet may grant via the public SDK.
var KnownCapabilities = map[string]struct{}{
	"torrents.read":       {},
	"torrents.add":        {},
	"torrents.actions":    {},
	"files.read":          {},
	"files.write":         {},
	"files.unpack":        {},
	"files.mediainfo":     {},
	"files.screenshots":   {},
	"files.spectrogram":   {},
	"trackers.read":       {},
	"trackers.write":      {},
	"peers.read":          {},
	"peers.actions":       {},
	"engine.capabilities": {},
	"process.exec":        {},
	"events.subscribe":    {},
	"jobs.schedule":       {},
	"jobs.read":           {},
	"storage.read":        {},
	"storage.write":       {},
	"settings.read":       {},
	"settings.write":      {},
	"system.read":         {},
	"notifications.write": {},
	"http.fetch":          {},
	"urlrewrite.read":     {},
	"urlrewrite.write":    {},
	"cookies.read":        {},
	"cookies.write":       {},
	"loginmgr.read":       {},
	"loginmgr.write":      {},
	"gateways.read":       {},
	"gateways.write":      {},
	"tools.mktorrent":     {},
}

// Manifest is the on-disk extension contract (no shell hooks allowed).
type Manifest struct {
	Schema       int       `yaml:"schema" json:"schema"`
	ID           string    `yaml:"id" json:"id"`
	Name         string    `yaml:"name" json:"name"`
	Description  string    `yaml:"description,omitempty" json:"description,omitempty"`
	Version      string    `yaml:"version" json:"version"`
	API          string    `yaml:"api" json:"api"`
	Engines      []string  `yaml:"engines" json:"engines,omitempty"`
	Capabilities []string  `yaml:"capabilities" json:"capabilities,omitempty"`
	Frontend     *Frontend `yaml:"frontend" json:"frontend,omitempty"`
	Backend      *Backend  `yaml:"backend" json:"backend,omitempty"`
}

// Frontend describes a declarative UI contribution package.
type Frontend struct {
	Type  string `yaml:"type" json:"type"`
	Entry string `yaml:"entry" json:"entry"`
}

// Backend describes an external worker process.
type Backend struct {
	Type       string `yaml:"type" json:"type"`
	Executable string `yaml:"executable" json:"executable"`
	Protocol   string `yaml:"protocol" json:"protocol"`
}

// ParseYAML unmarshals and validates a manifest document.
func ParseYAML(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks contractual constraints (OPEN-05).
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d (want %d)", m.Schema, SchemaVersion)
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("invalid id %q", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 120 {
		return fmt.Errorf("invalid name")
	}
	if len(m.Description) > 280 {
		return fmt.Errorf("description must be <= 280 characters")
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("invalid version %q", m.Version)
	}
	if strings.TrimSpace(m.API) == "" {
		return fmt.Errorf("api compatibility range required")
	}
	seenEngines := map[string]struct{}{}
	for _, e := range m.Engines {
		if _, ok := KnownEngines[e]; !ok {
			return fmt.Errorf("unknown engine %q", e)
		}
		if _, dup := seenEngines[e]; dup {
			return fmt.Errorf("duplicate engine %q", e)
		}
		seenEngines[e] = struct{}{}
	}
	seenCaps := map[string]struct{}{}
	for _, c := range m.Capabilities {
		if _, ok := KnownCapabilities[c]; !ok {
			return fmt.Errorf("unknown capability %q", c)
		}
		if _, dup := seenCaps[c]; dup {
			return fmt.Errorf("duplicate capability %q", c)
		}
		seenCaps[c] = struct{}{}
	}
	if m.Frontend == nil && m.Backend == nil {
		return fmt.Errorf("at least one of frontend or backend is required")
	}
	if m.Frontend != nil {
		if m.Frontend.Type != "declarative" {
			return fmt.Errorf("frontend.type must be declarative")
		}
		entry := strings.TrimSpace(m.Frontend.Entry)
		if entry == "" || strings.Contains(entry, "\x00") || !strings.HasSuffix(entry, ".json") {
			return fmt.Errorf("frontend.entry must be a .json path")
		}
		if strings.HasPrefix(entry, "/") || strings.Contains(entry, "..") {
			return fmt.Errorf("frontend.entry must be a relative path without ..")
		}
	}
	if m.Backend != nil {
		if m.Backend.Type != "process" {
			return fmt.Errorf("backend.type must be process")
		}
		exe := strings.TrimSpace(m.Backend.Executable)
		if exe == "" {
			return fmt.Errorf("backend.executable required")
		}
		if strings.HasPrefix(exe, "/") || strings.Contains(exe, "..") {
			return fmt.Errorf("backend.executable must be a relative path without ..")
		}
		if m.Backend.Protocol != "jsonrpc-2" {
			return fmt.Errorf("backend.protocol must be jsonrpc-2")
		}
	}
	return nil
}

// HasCapability reports whether the manifest requests a capability.
func (m *Manifest) HasCapability(cap string) bool {
	if m == nil {
		return false
	}
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}
