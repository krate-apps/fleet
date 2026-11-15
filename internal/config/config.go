package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds Fleet instance settings for a single Unix user.
type Config struct {
	Listen        ListenConfig   `yaml:"listen"`
	RTorrent      RTorrentConfig `yaml:"rtorrent"`
	Tools         ToolsConfig    `yaml:"tools"`
	GeoIP         GeoIPConfig    `yaml:"geoip"`
	Buckets       BucketsConfig  `yaml:"buckets"`
	Auth          AuthConfig     `yaml:"auth"`
	Security      SecurityConfig `yaml:"security"`
	InstallPrefix string         `yaml:"install_prefix"` // optional /opt/<user>/fleet for update helper hints
	Updates       UpdatesConfig  `yaml:"updates"`
	ExtensionsDir string         `yaml:"extensions_dir"`
	DataDir       string         `yaml:"data_dir"`
	LogLevel      string         `yaml:"log_level"`
}

// SecurityConfig hardens extension host capabilities.
type SecurityConfig struct {
	// ProcessExecAllowlist is the absolute binaries allowed for fleet.process.exec (fail closed when empty).
	ProcessExecAllowlist []string `yaml:"process_exec_allowlist"`
	// ProcessExecDirs allows any non-forbidden binary under these absolute directories.
	ProcessExecDirs []string `yaml:"process_exec_dirs"`
}

// UpdatesConfig controls GitHub release discovery for Settings → Updates.
type UpdatesConfig struct {
	// GitHubRepo is "owner/name" (default krate-apps/fleet).
	GitHubRepo string `yaml:"github_repo"`
	// AssetPattern is a filepath.Match pattern for the core release tarball
	// (default "fleet-*-linux-amd64.tgz").
	AssetPattern string `yaml:"asset_pattern"`
}

// AuthConfig controls native authentication (SEC-001…).
type AuthConfig struct {
	// Disable skips session enforcement (local automation only; not for production).
	Disable bool `yaml:"disable"`
	// SecureCookies forces the Secure flag on session cookies (enable behind HTTPS).
	SecureCookies bool `yaml:"secure_cookies"`
}

// BucketsConfig controls the optional official bucket (BKT-002).
type BucketsConfig struct {
	// OfficialPath is a directory containing bucket.json for the Fleet official bucket.
	OfficialPath string `yaml:"official_path"`
	// DisableOfficial skips pre-registration of the official bucket.
	DisableOfficial bool `yaml:"disable_official"`
}

type ListenConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type RTorrentConfig struct {
	// Address is a Unix socket path (unix:///path) or TCP endpoint (scgi://host:port).
	Address string `yaml:"address"`
	// DownloadRoots limits erase_data to paths under these absolute directories.
	DownloadRoots []string `yaml:"download_roots"`
}

// ToolsConfig points at optional JSON-capable helper binaries (CORE-014/015).
type ToolsConfig struct {
	Mktorrent   string `yaml:"mktorrent"`
	Dumptorrent string `yaml:"dumptorrent"`
	// Unrar is the CLI used for .rar extraction via fleet.files.unpack (empty = "unrar" on PATH).
	Unrar string `yaml:"unrar"`
	// Mediainfo is the CLI for media file inspection (empty = "mediainfo" on PATH).
	Mediainfo string `yaml:"mediainfo"`
	// Ffmpeg extracts video stills for fleet.files.screenshots (empty = "ffmpeg" on PATH).
	Ffmpeg string `yaml:"ffmpeg"`
	// Sox renders audio spectrograms for fleet.files.spectrogram (empty = "sox" on PATH).
	Sox string `yaml:"sox"`
	// Rclone is the CLI for org.fleet.rclone post-complete copy/move (empty = "rclone" on PATH).
	Rclone string `yaml:"rclone"`
}

// GeoIPConfig optionally loads a MaxMind Country MMDB for peer enrichment.
type GeoIPConfig struct {
	// Database is an absolute path to GeoLite2-Country.mmdb (or GeoIP2-Country).
	Database string `yaml:"database"`
}

// Default returns a starting configuration suitable for local development.
func Default() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Config{
		Listen: ListenConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		RTorrent: RTorrentConfig{
			Address: "",
		},
		Updates: UpdatesConfig{
			GitHubRepo:   "krate-apps/fleet",
			AssetPattern: "fleet-*-linux-amd64.tgz",
		},
		DataDir:  filepath.Join(home, ".local", "share", "fleet"),
		LogLevel: "info",
	}
}

// Load reads a YAML configuration file. Missing file yields Default().
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Listen.Port == 0 {
		cfg.Listen.Port = 8080
	}
	if cfg.Listen.Host == "" {
		cfg.Listen.Host = "127.0.0.1"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = Default().DataDir
	}
	if cfg.Updates.GitHubRepo == "" {
		cfg.Updates.GitHubRepo = "krate-apps/fleet"
	}
	if cfg.Updates.AssetPattern == "" {
		cfg.Updates.AssetPattern = "fleet-*-linux-amd64.tgz"
	}
	return cfg, nil
}

// Addr returns the HTTP listen address.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Listen.Host, c.Listen.Port)
}

// EnsureDataDir creates the Fleet data directory if needed.
func (c Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0o700)
}
