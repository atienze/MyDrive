package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all values loaded from config.toml at the project root.
type Config struct {
	ServerAddr string `toml:"server_addr"`
	Token      string `toml:"token"`
	SyncDir    string `toml:"sync_dir"`
	DeviceName string `toml:"device_name"`
}

// ConfigPath returns the absolute path to the config file (~/.vaultsync/config.toml).
// Exported so tests and CLI error messages can show the expected location.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".vaultsync", "config.toml"), nil
}

// StatePath returns the path to state.json, in the same directory as config.toml.
func StatePath() (string, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgPath), "state.json"), nil
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
				"  sync_dir    = \"<path-to-sync>\"\n"+
				"  device_name = \"<device-name>\"",
			path, path,
		)
	}

	var cfg Config

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", path, err)
	}

	if cfg.ServerAddr == "" {
		return nil, fmt.Errorf("config: server_addr is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("config: token is required")
	}
	if cfg.SyncDir == "" {
		return nil, fmt.Errorf("config: sync_dir is required")
	}

	return &cfg, nil
}
