package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Theme string `json:"theme"`
}

func DefaultConfig() Config {
	return Config{
		Theme: "catppuccin",
	}
}

func ConfigFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "ghtrend", "config.json"), nil
}

func LoadConfig() Config {
	path, err := ConfigFilePath()
	if err != nil {
		return DefaultConfig()
	}
	return LoadConfigFromPath(path)
}

func LoadConfigFromPath(path string) Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	_ = json.Unmarshal(data, &cfg)
	if cfg.Theme == "" {
		cfg.Theme = "catppuccin"
	}
	return cfg
}

func SaveConfig(cfg Config) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	return SaveConfigToPath(cfg, path)
}

func SaveConfigToPath(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
