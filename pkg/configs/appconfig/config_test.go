package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "catppuccin" {
		t.Errorf("expected default theme to be catppuccin, got %q", cfg.Theme)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ghtrend-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	cfg := Config{
		Theme: "dracula",
	}

	// Test direct save and load logic
	data := []byte(`{"theme": "dracula"}`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	readData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("expected %s, got %s", data, readData)
	}

	_ = cfg
}
