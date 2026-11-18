package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultDebounceMS   = 300
	DefaultMaxFileBytes = 50 * 1024 * 1024 // 50 MiB
)

// Config holds runtime settings for Strata.
type Config struct {
	DebounceMS   int    `json:"debounce_ms"`
	MaxFileBytes int64  `json:"max_file_bytes"`
	DataDir      string `json:"data_dir,omitempty"` // empty = default appdata dir
}

// Default returns built-in defaults.
func Default() Config {
	return Config{
		DebounceMS:   DefaultDebounceMS,
		MaxFileBytes: DefaultMaxFileBytes,
	}
}

// Path returns the config file path inside dataDir.
func Path(dataDir string) string {
	return filepath.Join(dataDir, "config.json")
}

// Load reads config from path. Missing file yields defaults (and writes them).
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := Save(path, cfg); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// Save writes cfg as indented JSON.
func Save(path string, cfg Config) error {
	cfg.applyDefaults()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) applyDefaults() {
	if c.DebounceMS <= 0 {
		c.DebounceMS = DefaultDebounceMS
	}
	if c.MaxFileBytes < 0 {
		c.MaxFileBytes = DefaultMaxFileBytes
	}
}

// Debounce returns the debounce duration.
func (c Config) Debounce() time.Duration {
	return time.Duration(c.DebounceMS) * time.Millisecond
}
