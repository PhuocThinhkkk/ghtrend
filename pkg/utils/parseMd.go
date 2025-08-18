package utils

import (
	"regexp"
	"strings"
)
func CleanMarkdown(input string) string {
    clean := input

    // Remove ANSI escape codes
    reANSI := regexp.MustCompile(`\x1b\[[0-9;]*m`)
    clean = reANSI.ReplaceAllString(clean, "")

    // Remove images (inline and linked)
    reImages := regexp.MustCompile(`\[!\[.*?\]\(.*?\)\]\(.*?\)|!\[.*?\]\(.*?\)`)
    clean = reImages.ReplaceAllString(clean, "")

    // Remove HTML tags (including <img>, <a>)
    reHTML := regexp.MustCompile(`(?s)<[^>]+>`)
    clean = reHTML.ReplaceAllString(clean, "")

    // Remove markdown links but keep link text
    reLinks := regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
    clean = reLinks.ReplaceAllString(clean, "$1")

    // Remove tables (lines with pipes)
    reTables := regexp.MustCompile(`(?m)^.*\|.*\|.*$`)
    clean = reTables.ReplaceAllString(clean, "")

    // Remove fenced code blocks ```
    reFencedCode := regexp.MustCompile("(?s)```.*?```")
    clean = reFencedCode.ReplaceAllString(clean, "")

    // Remove inline code `code`
    reInlineCode := regexp.MustCompile("`[^`]+`")
    clean = reInlineCode.ReplaceAllString(clean, "")

    // Remove blockquotes
    reBlockquote := regexp.MustCompile(`(?m)^\s*>.*$`)
    clean = reBlockquote.ReplaceAllString(clean, "")

    // Remove horizontal rules like --- or ***
    reHR := regexp.MustCompile(`(?m)^\s*([-*_]){3,}\s*$`)
    clean = reHR.ReplaceAllString(clean, "")

    // Remove emphasis markers
    reEmphasis := regexp.MustCompile(`(\*\*|__|\*|_)`)
    clean = reEmphasis.ReplaceAllString(clean, "")

    // Remove footnotes definitions and references
    reFootnoteDef := regexp.MustCompile(`(?m)^\s*\[\d+\]:\s+.*$`)
    clean = reFootnoteDef.ReplaceAllString(clean, "")
    reFootnoteRef := regexp.MustCompile(`\[\d+\]`)
    clean = reFootnoteRef.ReplaceAllString(clean, "")

    // Collapse multiple blank lines to max two
    reMultiNewline := regexp.MustCompile(`\n{3,}`)
    clean = reMultiNewline.ReplaceAllString(clean, "\n\n")

	clean = normalizeNewlines(clean)
    return clean
}

func normalizeNewlines(input string) string {
	// Step 1: Normalize all lines - remove those that contain only whitespace
	lines := strings.Split(input, "\n")
	var cleanedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleanedLines = append(cleanedLines, line)
		} else {
			// Keep a true empty line for visual spacing (optional)
			cleanedLines = append(cleanedLines, "")
		}
	}

	normalized := strings.Join(cleanedLines, "\n")

	// Step 2: Collapse multiple blank lines to a maximum of 1
	reMultiBlank := regexp.MustCompile(`(\n\s*\n){2,}`)
	normalized = reMultiBlank.ReplaceAllString(normalized, "\n\n")

	return normalized
}
