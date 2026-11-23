package utils

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

// ValidateBrowserURL ensures the URL is a valid absolute HTTP or HTTPS URL with a non-empty host.
func ValidateBrowserURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid or non-http url: %q", rawURL)
	}
	return nil
}

// OpenBrowser opens the specified URL in the system's default browser after validating the URL.
func OpenBrowser(rawURL string) error {
	if err := ValidateBrowserURL(rawURL); err != nil {
		return err
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Prevent zombie processes on Unix systems by reaping exit status in background
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}
