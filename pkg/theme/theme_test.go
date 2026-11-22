package theme

import (
	"testing"
)

func TestCatalogNotEmpty(t *testing.T) {
	if len(Catalog) == 0 {
		t.Fatalf("expected non-empty theme catalog")
	}
}

func TestGetTheme(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"catppuccin", "catppuccin"},
		{"Catppuccin Mocha", "catppuccin"},
		{"DRACULA", "dracula"},
		{"tokyonight", "tokyonight"},
		{"nord", "nord"},
		{"gruvbox", "gruvbox"},
		{"cyberpunk", "cyberpunk"},
		{"classic", "classic"},
		{"unknown-theme", "catppuccin"}, // fallback to default
	}

	for _, tt := range tests {
		th := GetTheme(tt.input)
		if th.ID != tt.expected {
			t.Errorf("GetTheme(%q) = %q, want %q", tt.input, th.ID, tt.expected)
		}
	}
}

func TestAvailableThemeIDs(t *testing.T) {
	ids := AvailableThemeIDs()
	if len(ids) != len(Catalog) {
		t.Errorf("expected %d IDs, got %d", len(Catalog), len(ids))
	}
}
