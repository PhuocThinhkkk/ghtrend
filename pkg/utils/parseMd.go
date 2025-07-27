package utils

import (
	"regexp"
	"strings"
)
func CleanMarkdown(input string) string {
    clean := input

    // Remove ANSI escapes (optional)
    ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
    clean = ansi.ReplaceAllString(clean, "")

    // Remove images
    img := regexp.MustCompile(`!\[.*?\]\(.*?\)`)
    clean = img.ReplaceAllString(clean, "")

    // Replace links [text](url) with [text]
    link := regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
    clean = link.ReplaceAllString(clean, "[$1]")

    // Remove footnote definitions and references
    refDef := regexp.MustCompile(`(?m)^\s*\[\d+\]:\s+.*$`)
    clean = refDef.ReplaceAllString(clean, "")
    refUse := regexp.MustCompile(`\[(.*?)\]\[\d+\]`)
    clean = refUse.ReplaceAllString(clean, "[$1]")

    // Remove HTML tags
    html := regexp.MustCompile(`(?s)<[^>]+>`)
    clean = html.ReplaceAllString(clean, "")

    // Remove any line containing two or more pipes
    anyTable := regexp.MustCompile(`(?m)^.*\|.+\|.*$`)
    clean = anyTable.ReplaceAllString(clean, "")

    // Remove leading or trailing pipes on any line
    leadPipe := regexp.MustCompile(`(?m)^\s*\|\s*`)
    clean = leadPipe.ReplaceAllString(clean, "")
    trailPipe := regexp.MustCompile(`(?m)\s*\|\s*$`)
    clean = trailPipe.ReplaceAllString(clean, "")

    // Remove fenced code blocks ```
    fences := regexp.MustCompile("(?s)```.*?```")
    clean = fences.ReplaceAllString(clean, "")

    // Remove inline code `code`
    inlineCode := regexp.MustCompile("`[^`]+`")
    clean = inlineCode.ReplaceAllString(clean, "")

    // Remove blockquotes
    bq := regexp.MustCompile(`(?m)^\s*>.*$`)
    clean = bq.ReplaceAllString(clean, "")

    // Remove horizontal rules like --- or ***
    hr := regexp.MustCompile(`(?m)^\s*([-*_]){3,}\s*$`)
    clean = hr.ReplaceAllString(clean, "")

    // Remove bold and italics markers *, **, _, __
    em := regexp.MustCompile(`(\*\*|__|\*|_)`)
    clean = em.ReplaceAllString(clean, "")

    // Collapse multiple blank lines to max two
    clean = regexp.MustCompile(`\n{3,}`).ReplaceAllString(clean, "\n\n")
    clean = strings.TrimSpace(clean)

	re := regexp.MustCompile(` {10,}`)
	clean =  re.ReplaceAllString(clean, " ")

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
