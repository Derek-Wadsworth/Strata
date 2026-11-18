package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Dir returns the per-user Strata data directory.
// Windows: %APPDATA%\Strata
// macOS/Linux: ~/.strata
func Dir() (string, error) {
	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("user config dir: %w", err)
		}
		return filepath.Join(base, "Strata"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".strata"), nil
}

// EnsureDir creates the data directory if needed and returns its path.
func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return dir, nil
}

// DBPath is the SQLite database path inside the data directory.
func DBPath(dataDir string) string {
	return filepath.Join(dataDir, "strata.db")
}

// ConfigPath is the JSON config path inside the data directory.
func ConfigPath(dataDir string) string {
	return filepath.Join(dataDir, "config.json")
}

// LogPath is the daemon log file path inside the data directory.
func LogPath(dataDir string) string {
	return filepath.Join(dataDir, "strata.log")
}
