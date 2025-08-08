package ui

import (
	"github.com/charmbracelet/lipgloss"
	"io"
	"errors"
	"ghtrend/pkg/utils"
	"strings"
	"github.com/charmbracelet/glamour"
)

var (
	borderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4")).
	Padding(2, 2).
	Width(70).
	Height(32)

	headerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#5CE1E6")).
	Bold(true)

	renderer, _ = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(60),
		glamour.WithPreservedNewLines(),
	)
)

func RenderReadme(markdown string) (string, error) {
	header := headerStyle.Render("Readme Preview: ")

	if len(strings.TrimSpace(markdown)) == 0 {
		markdown = "_No README found._"
	} else {
		markdown = utils.CleanMarkdown(markdown)

		maxLines := 22
		lines := strings.Split(markdown, "\n")
		inx := 0
		countLine := 0
		for countLine <= maxLines {
			if(len(lines) <= inx ) {
				break
			}
			countLine += len(lines[inx])/60 + 1
			inx++
		}
		lines = append(lines[:(inx - 1)], "...") 
		markdown = strings.Join(lines, "\n")

	}


	body, err := renderer.Render(markdown)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	right := borderStyle.Render(content)
	return right, nil;
}
