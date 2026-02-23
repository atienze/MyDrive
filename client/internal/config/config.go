package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all values loaded from config.toml at the project root.
type Config struct {
	ServerAddr          string `toml:"server_addr"`
	Token               string `toml:"token"`
	WatchDir            string `toml:"watch_dir"`
	SyncIntervalSeconds int    `toml:"sync_interval_seconds"`
}

// ConfigPath returns the absolute path to the config file.
// Exported so tests and CLI error messages can show the expected location.
func ConfigPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine executable path: %w", err)
	}
	// Walk up from the binary location to the project root (HomelabSecureSync).
	// During development the binary sits at the repo root, so this resolves correctly.
	projectRoot := filepath.Dir(exe)
	return filepath.Join(projectRoot, "config.toml"), nil
}

// Load reads and parses config.toml from the project root directory.
// Returns a clear, actionable error if the file is missing or malformed.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf(
			"config file not found at %s\n"+
				"Run 'vault-sync-server register <device-name>' on your server,\n"+
				"then create %s with the printed token:\n\n"+
				"  server_addr = \"<server-ip>:9000\"\n"+
				"  token       = \"<64-char-token>\"\n"+
				"  watch_dir   = \"<path-to-sync>\"",
			path, path,
		)
	}

	// Apply defaults before decoding — TOML overwrites only the keys present in the file.
	cfg := Config{
		SyncIntervalSeconds: 60,
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", path, err)
	}

	if cfg.ServerAddr == "" {
		return nil, fmt.Errorf("config: server_addr is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("config: token is required")
	}
	if cfg.WatchDir == "" {
		return nil, fmt.Errorf("config: watch_dir is required")
	}

	return &cfg, nil
}
