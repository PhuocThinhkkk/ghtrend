package flags

import (
	"testing"
)

func TestNewConfig_Valid(t *testing.T) {
	cfg, err := NewConfig(true, 25, Daily, "Go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsCache {
		t.Errorf("expected IsCache true, got false")
	}
	if cfg.Limit != 25 {
		t.Errorf("expected Limit 25, got %d", cfg.Limit)
	}
	if cfg.Since != Daily {
		t.Errorf("expected Since 'daily', got %s", cfg.Since)
	}
	if cfg.Language != "Go" {
		t.Errorf("expected Language 'Go', got %s", cfg.Language)
	}
	if cfg.User != "" {
		t.Errorf("expected empty User, got %s", cfg.User)
	}
}

func TestNewConfig_InvalidFrequency(t *testing.T) {
	_, err := NewConfig(true, 25, Frequency("yearly"), "Go")
	if err == nil {
		t.Errorf("expected error for invalid frequency, got nil")
	}
}

func TestNewConfig_InvalidLimit(t *testing.T) {
	_, err := NewConfig(true, 0, Weekly, "Rust")
	if err == nil {
		t.Errorf("expected error for zero limit, got nil")
	}

	_, err = NewConfig(true, -5, Monthly, "Rust")
	if err == nil {
		t.Errorf("expected error for negative limit, got nil")
	}
}

func TestIsValidFrequency(t *testing.T) {
	valid := []Frequency{Daily, Weekly, Monthly}
	for _, f := range valid {
		if !IsValidFrequency(f) {
			t.Errorf("expected %s to be valid", f)
		}
	}

	invalid := []Frequency{"yearly", "hourly", "never", ""}
	for _, f := range invalid {
		if IsValidFrequency(f) {
			t.Errorf("expected %s to be invalid", f)
		}
	}
}
