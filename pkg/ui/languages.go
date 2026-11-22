package ui

import (
	"fmt"
	"sort"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderLanguagesBreakDown(totalWidth, height int, th theme.Theme) string {
	if totalWidth <= 0 {
		totalWidth = 44
	}
	lineWidth := max(10, totalWidth-4)
	header := renderHeader(lineWidth, "Languages Breakdown", th)
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
	if height > 0 {
		maxRows = max(1, height-3)
	}

	i := 0
	for _, p := range sortedPairs {
		i++
		languageStrings += renderEachField(lineWidth, p.lang, fmt.Sprintf("%s", p.lines))
		if i >= maxRows {
			break
		}
	}
	if i == 0 {
		languageStrings += renderEachField(lineWidth, "None", "0%")
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(th.Border).
		Padding(0, 1).
		Width(max(10, totalWidth-2))

	if height > 0 {
		borderStyle = borderStyle.Height(max(1, height-2)).MaxHeight(height)
	}

	return borderStyle.Render(header + languageStrings)
}
