package ui

import (
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func renderEachField(totalWidth int, label string, value string) string {
	spaceCount := totalWidth - lipgloss.Width(label) - lipgloss.Width(value) - 2
	if spaceCount < 1 {
		spaceCount = 1
	}

	s := label + strings.Repeat(" ", spaceCount) + value + "\n"
	return s
}

func renderHeader(totalWidth int, header string, th theme.Theme) string {
	headerBgStyle := lipgloss.NewStyle().
		Background(th.Primary).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Background)

	styledHeader := headerBgStyle.Render(headerStyle.Render(header))

	fillCount := totalWidth - lipgloss.Width(styledHeader) - 2
	fillCount = max(fillCount, 0)

	fill := lipgloss.NewStyle().Foreground(th.Border).Render(strings.Repeat("─", fillCount))

	return styledHeader + fill
}
