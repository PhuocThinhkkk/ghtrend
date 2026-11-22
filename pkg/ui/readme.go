package ui

import (
	"github.com/charmbracelet/lipgloss"
	"io"
	"errors"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
	"strings"
	"github.com/charmbracelet/glamour"
)

var (

	contentStyle = lipgloss.NewStyle().
	Padding(0, 2).
	Width(90).
	MaxHeight(32).
	Height(32)

	// Define your outer border style
	borderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4"))

	headerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#A5FFD6")).
	Italic(true).
	Underline(true).
	MarginLeft(1).
	MarginBottom(1)

	renderer, _ = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
		glamour.WithPreservedNewLines(),
	)
)

func RenderReadme(markdown string, width, height int, state activeComponent) (string, error) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	header := headerStyle.Render("Readme Preview:")

	if len(strings.TrimSpace(markdown)) == 0 {
		markdown = "_No README found._"
	} else {
		markdown = utils.CleanMarkdown(markdown)
	}

	wrapWidth := width - 6
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	termRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrapWidth),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		termRenderer = renderer
	}

	body, err := termRenderer.Render(markdown)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	style := contentStyle.Width(width - 4).Height(height - 2).MaxHeight(height - 2)
	bStyle := borderStyle
	if state == readmeActive {
		bStyle = bStyle.BorderForeground(lipgloss.Color("#BA8E23"))
	}
	out := bStyle.Render(style.Render(content))
	return out, nil
}
