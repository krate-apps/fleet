package logs

import (
	"fmt"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

var allowedUnit = regexp.MustCompile(`^(fleet|rtorrent|deluge|qbittorrent|transmission)@[A-Za-z0-9_.-]+$`)

// ValidateUnit returns the unit if it matches the allowlist.
func ValidateUnit(unit string) (string, error) {
	unit = strings.TrimSpace(unit)
	unit = strings.TrimSuffix(unit, ".service")
	if unit == "" {
		return "", fmt.Errorf("unit required")
	}
	if !allowedUnit.MatchString(unit) {
		return "", fmt.Errorf("unit %q not allowed", unit)
	}
	return unit, nil
}

// InstanceUser returns the Unix account name for unit heuristics.
func InstanceUser(installPrefix string) string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	// /opt/<user>/fleet → user
	prefix := strings.TrimSpace(installPrefix)
	if prefix != "" {
		parts := strings.Split(filepath.Clean(prefix), string(filepath.Separator))
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == "opt" && parts[i+1] != "" {
				return parts[i+1]
			}
		}
	}
	return "fleet"
}

// DefaultUnitForKind builds kind@user (e.g. deluge@trevor).
func DefaultUnitForKind(kind, username string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	username = strings.TrimSpace(username)
	if username == "" {
		username = InstanceUser("")
	}
	switch kind {
	case "fleet", "rtorrent", "deluge", "qbittorrent", "transmission":
		return kind + "@" + username
	default:
		return ""
	}
}

// DefaultLogPathsForKind returns candidate log files under home (first existing wins).
func DefaultLogPathsForKind(kind, homeDir string) []string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	homeDir = filepath.Clean(strings.TrimSpace(homeDir))
	if homeDir == "" || homeDir == "." {
		return nil
	}
	switch kind {
	case "qbittorrent":
		return []string{
			filepath.Join(homeDir, ".config", "qBittorrent", "qBittorrent", "data", "logs", "qbittorrent.log"),
			filepath.Join(homeDir, ".local", "share", "qBittorrent", "logs", "qbittorrent.log"),
		}
	case "deluge":
		return []string{
			filepath.Join(homeDir, ".config", "deluge", "deluged.log"),
			filepath.Join(homeDir, ".config", "deluge", "deluge.log"),
		}
	case "transmission":
		return []string{
			filepath.Join(homeDir, ".config", "transmission-daemon", "transmission.log"),
			filepath.Join(homeDir, ".config", "transmission", "transmission.log"),
		}
	default:
		return nil
	}
}
