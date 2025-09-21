package utils

import (
	"net/url"
	"strings"
)


// DetectAndDecode tries to detect if a string looks like URL-encoded UTF-8
// and decodes it into normal text (e.g., Chinese characters).
// It returns the decoded string and a boolean indicating whether decoding was performed.
func DetectAndDecode(s string) (string, bool) {
	if !strings.Contains(s, "%") {
		return s, false
	}

	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s, false
	}

	if decoded != s {
		return decoded, true
	}

	return s, false
}
