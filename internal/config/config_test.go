package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingWritesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DebounceMS != DefaultDebounceMS || cfg.MaxFileBytes != DefaultMaxFileBytes {
		t.Fatalf("defaults = %+v", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file written: %v", err)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	want := Config{DebounceMS: 500, MaxFileBytes: 1024}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.DebounceMS != 500 || got.MaxFileBytes != 1024 {
		t.Fatalf("got %+v", got)
	}
}
