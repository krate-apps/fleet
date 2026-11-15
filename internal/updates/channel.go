package updates

import (
	"fmt"
	"strings"
)

const (
	ChannelStable = "stable"
	ChannelPre    = "pre"
	metaChannelKey = "update_channel"
)

// NormalizeChannel returns stable|pre or an error.
func NormalizeChannel(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", ChannelStable:
		return ChannelStable, nil
	case ChannelPre, "prerelease", "pre-release", "lab":
		return ChannelPre, nil
	default:
		return "", fmt.Errorf("channel must be %q or %q", ChannelStable, ChannelPre)
	}
}

func (m *Manager) channelOrDefault() string {
	ch, err := m.Channel()
	if err != nil || ch == "" {
		return ChannelStable
	}
	return ch
}

// Channel returns the persisted update channel (default stable).
func (m *Manager) Channel() (string, error) {
	var raw string
	err := m.store.DB().QueryRow(`SELECT value FROM meta WHERE key = ?`, metaChannelKey).Scan(&raw)
	if err != nil || strings.TrimSpace(raw) == "" {
		return ChannelStable, nil
	}
	return NormalizeChannel(raw)
}

// SetChannel persists the update channel for available-release filtering.
func (m *Manager) SetChannel(raw string) (string, error) {
	ch, err := NormalizeChannel(raw)
	if err != nil {
		return "", err
	}
	_, err = m.store.DB().Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		metaChannelKey, ch,
	)
	if err != nil {
		return "", err
	}
	m.channel = ch
	return ch, nil
}
