package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
)

var (
	languagesBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(lipgloss.Color("63")).
		Padding(0,1)
)

func (m *Model) renderLanguagesBreakDown(totalWidth int) string {
	if totalWidth <= 0 {
		totalWidth = 44
	}
	header := renderHeader(totalWidth, "Languages Breakdown")
	repo := m.getCursorRepo()
	languageStrings := ""
	type kv struct {
		lang  string
		lines string
	}
	var sortedPairs []kv
	for k, v := range repo.LanguagesBreakDown {
		sortedPairs = append(sortedPairs, kv{k, v})
	}

	sort.Slice(sortedPairs, func(i, j int) bool {
		return sortedPairs[i].lines > sortedPairs[j].lines
	})

	maxRows := 5
	i := 0
	for _, p := range sortedPairs {
		i++
		languageStrings += renderEachField(totalWidth, p.lang, fmt.Sprintf("%s", p.lines))
		if i >= maxRows {
			if len(sortedPairs) > maxRows {
				languageStrings += "...\n"
			}
			break
		}
	}
	if i == 0 {
		languageStrings += renderEachField(totalWidth, "None", "0%")
	}
	return languagesBorderStyle.Width(totalWidth).Render(header + languageStrings)
}
