package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	languagesBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(lipgloss.Color("63")).
		Padding(0,1)
)

func (m *Model) renderLanguagesBreakDown() string {
	//totalWidth := m.table.Width() - m.list.Width()
	totalWidth := 48
	header := renderHeader(totalWidth, "Languages BreakDown")
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

	i := 0
	for _, p := range sortedPairs {
		i++
		languageStrings += renderEachField(totalWidth, p.lang, fmt.Sprintf("%s", p.lines))
		if i >= 6 {
			languageStrings += "...\n"
			break
		}
	}
	if i == 0 {
		languageStrings += "..."
	}
	if i < 6 {
		languageStrings += strings.Repeat("\n", 7-i)
	}
	return languagesBorderStyle.Width(totalWidth).Render(header + languageStrings)

}
