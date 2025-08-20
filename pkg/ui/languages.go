package ui

import (
	"fmt"

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
	i := 0
	for lang, percent := range repo.LanguagesBreakDown {
		i += 1
		languageStrings += renderEachField(totalWidth, lang, fmt.Sprintf("%d", percent))
		if i >= 6 {
			languageStrings +=  "...\n"
			break
		}
	}
	return languagesBorderStyle.Width(totalWidth).Render(languageStrings)

}

