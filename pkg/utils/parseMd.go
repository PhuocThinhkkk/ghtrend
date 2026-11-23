package utils

import (
	"regexp"
	"strings"

	"github.com/forPelevin/gomoji"
)

var (
	reANSI          = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reImages        = regexp.MustCompile(`\[!\[.*?\]\(.*?\)\]\(.*?\)|!\[.*?\]\(.*?\)`)
	reHTML          = regexp.MustCompile(`(?s)<[^>]+>`)
	reLinks         = regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
	reTables        = regexp.MustCompile(`(?m)^.*\|.*\|.*$`)
	reFencedCode    = regexp.MustCompile("(?s)```.*?```")
	reInlineCode    = regexp.MustCompile("`[^`]+`")
	reBlockquote    = regexp.MustCompile(`(?m)^\s*>.*$`)
	reHR            = regexp.MustCompile(`(?m)^\s*([-*_]){3,}\s*$`)
	reEmphasis      = regexp.MustCompile(`(\*\*|__|\*|_)`)
	reFootnoteDef   = regexp.MustCompile(`(?m)^\s*\[\d+\]:\s+.*$`)
	reFootnoteRef   = regexp.MustCompile(`\[\d+\]`)
	reMultiNewline  = regexp.MustCompile(`\n{3,}`)
	reEmptyHeadings = regexp.MustCompile(`(?m)^\s*#{1,6}\s*$`)
	reMultiBlank    = regexp.MustCompile(`(\n\s*\n){2,}`)
	headingRegex    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	emojiRegex      = regexp.MustCompile(`[\x{1F300}-\x{1FAD6}\x{1F600}-\x{1F64F}]`)
)

func CleanMarkdown(input string) string {
	clean := input

	clean = reANSI.ReplaceAllString(clean, "")
	clean = reImages.ReplaceAllString(clean, "")
	clean = reHTML.ReplaceAllString(clean, "")
	clean = reLinks.ReplaceAllString(clean, "$1")
	clean = reTables.ReplaceAllString(clean, "")
	clean = reFencedCode.ReplaceAllString(clean, "")
	clean = reInlineCode.ReplaceAllString(clean, "")
	clean = reBlockquote.ReplaceAllString(clean, "")
	clean = reHR.ReplaceAllString(clean, "")
	clean = reEmphasis.ReplaceAllString(clean, "")
	clean = reFootnoteDef.ReplaceAllString(clean, "")
	clean = reFootnoteRef.ReplaceAllString(clean, "")
	clean = reEmptyHeadings.ReplaceAllString(clean, "")
	clean = reMultiNewline.ReplaceAllString(clean, "\n\n")

	clean = normalizeNewlines(clean)
	clean = deduplicateConsecutiveHeadings(clean)
	clean = removeEmoji(clean)
	clean = removeEmojiLib(clean)
	return clean
}

func deduplicateConsecutiveHeadings(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	var lastHeadingText string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		match := headingRegex.FindStringSubmatch(trimmed)
		if len(match) == 3 {
			headingText := strings.ToLower(strings.TrimSpace(match[2]))
			if headingText != "" && headingText == lastHeadingText {
				continue
			}
			lastHeadingText = headingText
		} else if trimmed != "" {
			lastHeadingText = ""
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func normalizeNewlines(input string) string {
	lines := strings.Split(input, "\n")
	var cleanedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleanedLines = append(cleanedLines, line)
		} else {
			cleanedLines = append(cleanedLines, "")
		}
	}

	normalized := strings.Join(cleanedLines, "\n")
	normalized = reMultiBlank.ReplaceAllString(normalized, "\n\n")

	return normalized
}

func removeEmojiLib(s string) string {
	return gomoji.RemoveEmojis(s)
}

func removeEmoji(s string) string {
	return emojiRegex.ReplaceAllString(s, "")
}
