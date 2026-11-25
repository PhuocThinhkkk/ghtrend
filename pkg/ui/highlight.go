package ui

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const maxHighlightLines = 4000

// chromaRegistryMu serializes access to chroma's global style registry because
// glamour's CodeBlockElement.Render calls styles.Register without locking.
var chromaRegistryMu sync.Mutex

// chromaStyleName maps a ghtrend theme to a Chroma v2 style name with a safe fallback.
func chromaStyleName(th theme.Theme) string {
	switch strings.ToLower(strings.TrimSpace(th.ID)) {
	case "catppuccin":
		return "catppuccin-mocha"
	case "tokyonight", "tokyo-night":
		return "tokyonight-night"
	case "dracula":
		return "dracula"
	case "gruvbox":
		return "gruvbox"
	case "nord":
		return "nord"
	case "solarized-dark":
		return "solarized-dark"
	case "cyberpunk":
		return "witchhazel"
	case "classic":
		return "monokai"
	default:
		return "monokai"
	}
}

func resolveChromaStyle(th theme.Theme) *chroma.Style {
	name := chromaStyleName(th)
	chromaRegistryMu.Lock()
	defer chromaRegistryMu.Unlock()

	style := styles.Get(name)
	if style == nil || (style == styles.Fallback && name != "fallback") {
		style = styles.Get("monokai")
	}
	if style == nil {
		return styles.Fallback
	}
	return style
}

// RenderHighlightedCode highlights source code for filename using Chroma v2,
// normalizes CRLF/tabs, caps output at 4000 lines, and prepends a styled
// right-aligned line-number gutter.
func RenderHighlightedCode(filename, source string, th theme.Theme, maxWidth int) string {
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.ReplaceAll(normalized, "\t", "    ")

	rawLines := strings.Split(normalized, "\n")
	// Drop a single trailing empty element produced by a final newline in source files.
	if len(rawLines) > 1 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	totalLines := len(rawLines)
	truncated := totalLines > maxHighlightLines
	if truncated {
		rawLines = rawLines[:maxHighlightLines]
	}
	cappedSource := strings.Join(rawLines, "\n")

	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Analyse(cappedSource)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	style := resolveChromaStyle(th)

	formatted := cappedSource
	if iter, err := lexer.Tokenise(nil, cappedSource); err == nil {
		var buf strings.Builder
		if err := formatter.Format(&buf, style, iter); err == nil {
			formatted = strings.TrimSuffix(buf.String(), "\n")
		}
	}

	mutedColor := th.Muted
	if mutedColor == "" {
		mutedColor = lipgloss.Color("241")
	}
	borderColor := th.Border
	if borderColor == "" {
		borderColor = lipgloss.Color("240")
	}

	numStyle := lipgloss.NewStyle().Foreground(mutedColor)
	sepStyle := lipgloss.NewStyle().Foreground(borderColor)
	sep := sepStyle.Render("│ ")

	formattedLines := strings.Split(formatted, "\n")
	outLines := make([]string, 0, len(formattedLines)+1)
	for i, line := range formattedLines {
		gutter := numStyle.Render(fmt.Sprintf(" %4d ", i+1)) + sep
		fullLine := gutter + line
		if maxWidth > 0 {
			fullLine = TruncateDisplayWidth(fullLine, maxWidth, "…")
		}
		outLines = append(outLines, fullLine)
	}

	if truncated {
		footer := fmt.Sprintf("... (%d more lines truncated, showing first %d of %d lines)", totalLines-maxHighlightLines, maxHighlightLines, totalLines)
		if maxWidth > 0 {
			footer = TruncateDisplayWidth(footer, maxWidth, "…")
		}
		outLines = append(outLines, numStyle.Render(footer))
	}

	return strings.Join(outLines, "\n")
}

// RenderFullMarkdown renders Markdown with Glamour preserving fenced code blocks,
// tables, links, and formatting without stripping Markdown syntax.
func RenderFullMarkdown(rawMD string, th theme.Theme, wrapWidth int) string {
	if strings.TrimSpace(rawMD) == "" {
		return ""
	}

	wrapWidth = max(20, wrapWidth)

	glamourStyle := strings.TrimSpace(th.Glamour)
	if glamourStyle == "" {
		glamourStyle = "dark"
	}

	termRenderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(wrapWidth),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		termRenderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(wrapWidth),
			glamour.WithPreservedNewLines(),
		)
		if err != nil || termRenderer == nil {
			return rawMD
		}
	}

	chromaRegistryMu.Lock()
	body, err := termRenderer.Render(rawMD)
	chromaRegistryMu.Unlock()
	if err != nil && !errors.Is(err, io.EOF) {
		return rawMD
	}

	return strings.TrimRight(body, "\n")
}
