package appconfig

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "catppuccin" {
		t.Errorf("expected default theme to be catppuccin, got %q", cfg.Theme)
	}
}

func TestConfigFilePath(t *testing.T) {
	path, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("unexpected error getting config file path: %v", err)
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("expected filename 'config.json', got %s", filepath.Base(path))
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ghtrend", "config.json")

	cfg := Config{
		Theme: "dracula",
	}

	err := SaveConfigToPath(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error saving config: %v", err)
	}

	loaded := LoadConfigFromPath(configPath)
	if loaded.Theme != "dracula" {
		t.Errorf("expected theme 'dracula', got %q", loaded.Theme)
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	loaded := LoadConfigFromPath(configPath)
	if loaded.Theme != "catppuccin" {
		t.Errorf("expected fallback theme 'catppuccin', got %q", loaded.Theme)
	}
}
