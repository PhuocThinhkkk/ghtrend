package ui

import (
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func renderEachField(lineWidth int, label string, value string) string {
	spaceCount := lineWidth - lipgloss.Width(label) - lipgloss.Width(value)
	if spaceCount < 1 {
		spaceCount = 1
	}

	return label + strings.Repeat(" ", spaceCount) + value + "\n"
}

func renderEachFieldWithTheme(lineWidth int, label string, value string, th theme.Theme) string {
	styledLabel := lipgloss.NewStyle().Foreground(th.Muted).Render(label)
	styledVal := lipgloss.NewStyle().Foreground(th.Text).Bold(true).Render(value)
	spaceCount := lineWidth - lipgloss.Width(label) - lipgloss.Width(value)
	if spaceCount < 1 {
		spaceCount = 1
	}

	return styledLabel + strings.Repeat(" ", spaceCount) + styledVal + "\n"
}

func renderHeader(lineWidth int, header string, th theme.Theme) string {
	headerBgStyle := lipgloss.NewStyle().
		Background(th.Primary).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Background)

	styledHeader := headerBgStyle.Render(headerStyle.Render(header))

	fillCount := lineWidth - lipgloss.Width(styledHeader)
	fillCount = max(fillCount, 0)

	fill := lipgloss.NewStyle().Foreground(th.Border).Render(strings.Repeat("─", fillCount))

	return styledHeader + fill + "\n"
}
