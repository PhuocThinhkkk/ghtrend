package utils

import (
	"regexp"
	"strings"
)

// CleanMarkdown sanitizes markdown for terminal rendering
	// 1. Remove images: ![alt](url)
	// 2. Replace links: [text](url) => [text]
	// 3. Remove footnote-style links: [text][1] and [1]: http...
	// 4. Remove raw HTML blocks
	// 5. Optional: remove Markdown tables
	// 6. Normalize whitespace
func CleanMarkdown(input string) string {
	clean := input

	imgPattern := regexp.MustCompile(`!\[.*?\]\(.*?\)`)
	clean = imgPattern.ReplaceAllString(clean, "")

	linkPattern := regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
	clean = linkPattern.ReplaceAllString(clean, "[$1]")

	refPattern := regexp.MustCompile(`(?m)^\s*\[\d+\]:\s+.*$`)
	clean = refPattern.ReplaceAllString(clean, "")
	labelRef := regexp.MustCompile(`\[(.*?)\]\[\d+\]`)
	clean = labelRef.ReplaceAllString(clean, "[$1]")

	htmlPattern := regexp.MustCompile(`(?s)<[^>]+>`)
	clean = htmlPattern.ReplaceAllString(clean, "")

	tablePattern := regexp.MustCompile(`(?m)^\s*\|.*\|.*$`)
	clean = tablePattern.ReplaceAllString(clean, "")

	clean = strings.TrimSpace(clean)

	return clean
}

