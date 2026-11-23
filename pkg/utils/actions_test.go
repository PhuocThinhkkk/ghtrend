package utils

import (
	"testing"
)

func TestOpenBrowser_Validation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty url", "", true},
		{"relative url", "/foo/bar", true},
		{"file scheme", "file:///etc/passwd", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"ftp scheme", "ftp://example.com", true},
		{"empty host", "https://", true},
		{"valid http", "http://github.com/PhuocThinhkkk/ghtrend", false},
		{"valid https", "https://github.com/PhuocThinhkkk/ghtrend", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBrowserURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBrowserURL(%q) err = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}

	// Smoke test that OpenBrowser returns an error immediately for invalid URLs
	if err := OpenBrowser("invalid-url"); err == nil {
		t.Errorf("OpenBrowser('invalid-url') expected error, got nil")
	}
}

func TestCopyToClipboard_Smoke(t *testing.T) {
	// Verify CopyToClipboard does not panic on arbitrary strings
	_ = CopyToClipboard("https://github.com/PhuocThinhkkk/ghtrend?tab=readme&since=daily")
}
