package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
)

var (
	languagesBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(lipgloss.Color("63"))
)

func (m *Model) renderLanguagesBreakDown() string {
	//totalWidth := m.table.Width() - m.list.Width()
	totalWidth := 40
	repo := m.getCursorRepo()
	languageStrings := ""
	type kv struct {
		lang  string
		lines int
	}
	var sortedPairs []kv
	for k, v := range repo.LanguagesBreakDown {
		sortedPairs = append(sortedPairs, kv{k, v})
	}

	sort.Slice(sortedPairs, func(i, j int) bool {
		return sortedPairs[i].lines > sortedPairs[j].lines
	})

	i := 0
	for _, p := range sortedPairs {
		i += 1
		languageStrings += renderEachField(totalWidth, p.lang, fmt.Sprintf("%d", p.lines))
		if i >= 6 {
			languageStrings += "...\n"
			break
		}
	}
	return languagesBorderStyle.Width(totalWidth).Render(languageStrings)

}
