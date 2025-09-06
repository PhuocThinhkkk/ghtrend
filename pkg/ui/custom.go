package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)


func renderEachField (totalWidth int, label string, value string ) string {
	spaceCount := totalWidth - lipgloss.Width(label) - lipgloss.Width(value) - 2

	s := label + strings.Repeat(" ", spaceCount) + value + "\n"
	return s

}

func renderHeader(totalWidth int, header string) string {
	//FIX:  why is the word not being style when i set Backgound color?
	headerBgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#00A7FF")). // blue bg
		Padding(0, 1)                          // space left/right
	
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))

	styledHeader := headerBgStyle.Render(headerStyle.Render(header))

	fillCount := totalWidth - lipgloss.Width(styledHeader) - 2
	fillCount = max(fillCount, 0)

	fill := strings.Repeat("─", fillCount)

	return styledHeader + fill
}
