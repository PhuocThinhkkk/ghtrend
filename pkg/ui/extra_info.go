package ui

import (
	"strings"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	
	extraStyle = lipgloss.NewStyle().
    Border(lipgloss.NormalBorder(), true).
    BorderForeground(lipgloss.Color("63")) 

)

func renderEachField (totalWidth int, label string, value string ) string {
	spaceCount := totalWidth - lipgloss.Width(label) - lipgloss.Width(value)

	s := label + strings.Repeat(" ", spaceCount) + value + "\n"
	return s

}


func (m *Model) renderExtraInfor() string {
	//totalWidth := m.table.Width() - m.list.Width()
	totalWidth := 40
	repo := m.getCursorRepo()

	watchers := renderEachField(totalWidth, "watchers:", fmt.Sprintf("%d", repo.ExtraInfor.Watchers))
	openissues := renderEachField(totalWidth, "open issues:", fmt.Sprintf("%d", repo.ExtraInfor.OpenIssues))
	SubscribersCount := renderEachField(totalWidth, "subscribers: ", fmt.Sprintf("%d", repo.ExtraInfor.SubscribersCount))
	size := renderEachField(totalWidth, "size: ", fmt.Sprintf("%d",repo.ExtraInfor.Size) + "KB")
	return extraStyle.Width(totalWidth).Render(size + watchers + openissues + SubscribersCount )

}
