package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderLanguagesBreakDown(totalWidth, height int, th theme.Theme) string {
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
		pct   float64
	}
	var sortedPairs []kv
	for k, v := range repo.LanguagesBreakDown {
		clean := strings.TrimSpace(strings.TrimSuffix(v, "%"))
		val, _ := strconv.ParseFloat(clean, 64)
		sortedPairs = append(sortedPairs, kv{lang: k, lines: v, pct: val})
	}

	sort.Slice(sortedPairs, func(i, j int) bool {
		return sortedPairs[i].pct > sortedPairs[j].pct
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
		Width(max(10, totalWidth-2)).
		MaxWidth(totalWidth)

	if height > 0 {
		borderStyle = borderStyle.Height(max(1, height-2)).MaxHeight(height)
	}

	content := strings.TrimRight(header+languageStrings, "\n")
	return borderStyle.Render(content)
}
