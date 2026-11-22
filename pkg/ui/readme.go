package ui

import (
	"errors"
	"io"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

func RenderReadme(markdown string, width, height int, state activeComponent, th theme.Theme) (string, error) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(th.Accent).
		Bold(true).
		MarginLeft(1).
		MarginBottom(1)

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

	glamourStyle := th.Glamour
	if glamourStyle == "" {
		glamourStyle = "dark"
	}

	termRenderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(wrapWidth),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		termRenderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(wrapWidth),
			glamour.WithPreservedNewLines(),
		)
	}

	body, err := termRenderer.Render(markdown)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	contentStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 4).
		Height(height - 2).
		MaxHeight(height - 2)

	bStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Border)

	if state == readmeActive {
		bStyle = bStyle.BorderForeground(th.FocusBorder)
	}

	out := bStyle.Render(contentStyle.Render(content))
	return out, nil
}
