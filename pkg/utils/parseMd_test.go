package utils

import (
	"strings"
	"testing"
)

func TestCleanMarkdown_DeduplicateConsecutiveHeadings(t *testing.T) {
	input := `# WPGraphQL
# WPGraphQL
# WPGraphQL

Unlock the power of WordPress data with GraphQL
`
	cleaned := CleanMarkdown(input)
	count := strings.Count(cleaned, "WPGraphQL")
	if strings.Contains(cleaned, "# WPGraphQL\n# WPGraphQL") {
		t.Errorf("expected consecutive duplicate headings to be deduplicated, got:\n%s", cleaned)
	}
	if count != 1 {
		t.Errorf("expected 1 occurrence of WPGraphQL heading, got %d:\n%s", count, cleaned)
	}
}

func TestCleanMarkdown_RemovesImagesAndLinks(t *testing.T) {
	input := `[![Logo](logo.png)](https://example.com)
# Title
[Link](https://example.com)`

	cleaned := CleanMarkdown(input)
	if strings.Contains(cleaned, "logo.png") {
		t.Errorf("expected image markdown to be removed, got:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "Link") {
		t.Errorf("expected link text 'Link' to remain, got:\n%s", cleaned)
	}
}
