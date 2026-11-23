package utils

import (
	"github.com/atotto/clipboard"
)

// CopyToClipboard copies text to the system clipboard in a cross-platform, safe manner.
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}
