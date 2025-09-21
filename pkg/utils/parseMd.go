package utils

import (
	"github.com/forPelevin/gomoji"
	"regexp"
	"strings"
)

func CleanMarkdown(input string) string {
	clean := input

	reANSI := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	clean = reANSI.ReplaceAllString(clean, "")

	reImages := regexp.MustCompile(`\[!\[.*?\]\(.*?\)\]\(.*?\)|!\[.*?\]\(.*?\)`)
	clean = reImages.ReplaceAllString(clean, "")

	reHTML := regexp.MustCompile(`(?s)<[^>]+>`)
	clean = reHTML.ReplaceAllString(clean, "")

	reLinks := regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
	clean = reLinks.ReplaceAllString(clean, "$1")

	reTables := regexp.MustCompile(`(?m)^.*\|.*\|.*$`)
	clean = reTables.ReplaceAllString(clean, "")

	reFencedCode := regexp.MustCompile("(?s)```.*?```")
	clean = reFencedCode.ReplaceAllString(clean, "")

	reInlineCode := regexp.MustCompile("`[^`]+`")
	clean = reInlineCode.ReplaceAllString(clean, "")

	reBlockquote := regexp.MustCompile(`(?m)^\s*>.*$`)
	clean = reBlockquote.ReplaceAllString(clean, "")

	reHR := regexp.MustCompile(`(?m)^\s*([-*_]){3,}\s*$`)
	clean = reHR.ReplaceAllString(clean, "")

	reEmphasis := regexp.MustCompile(`(\*\*|__|\*|_)`)
	clean = reEmphasis.ReplaceAllString(clean, "")

	reFootnoteDef := regexp.MustCompile(`(?m)^\s*\[\d+\]:\s+.*$`)
	clean = reFootnoteDef.ReplaceAllString(clean, "")
	reFootnoteRef := regexp.MustCompile(`\[\d+\]`)
	clean = reFootnoteRef.ReplaceAllString(clean, "")

	reMultiNewline := regexp.MustCompile(`\n{3,}`)
	clean = reMultiNewline.ReplaceAllString(clean, "\n\n")

	clean = normalizeNewlines(clean)
	clean = removeEmoji(clean)
	clean = removeEmojiLib(clean)
	return clean
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

	reMultiBlank := regexp.MustCompile(`(\n\s*\n){2,}`)
	normalized = reMultiBlank.ReplaceAllString(normalized, "\n\n")

	return normalized
}

var emojiRegex = regexp.MustCompile(`[\x{1F300}-\x{1FAD6}\x{1F600}-\x{1F64F}]`)

func removeEmojiLib(s string) string {
	return gomoji.RemoveEmojis(s)
}

func removeEmoji(s string) string {
	return emojiRegex.ReplaceAllString(s, "")
}
