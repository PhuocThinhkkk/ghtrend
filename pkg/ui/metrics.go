package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	
	extraStyle = lipgloss.NewStyle().
    Border(lipgloss.NormalBorder(), true).
    BorderForeground(lipgloss.Color("63")).
	Padding(0, 1)

)


func (m *Model) renderExtraInfor() string {
	//totalWidth := m.table.Width() - m.list.Width()
	totalWidth := 48
	repo := m.getCursorRepo()

	header := renderHeader(totalWidth, "Project Metrics")
	watchers := renderEachField(totalWidth, "watchers:", fmt.Sprintf("%d", repo.ExtraInfor.Watchers))
	openissues := renderEachField(totalWidth, "open issues:", fmt.Sprintf("%d", repo.ExtraInfor.OpenIssues))
	SubscribersCount := renderEachField(totalWidth, "subscribers: ", fmt.Sprintf("%d", repo.ExtraInfor.SubscribersCount))
	size := renderEachField(totalWidth, "size: ", fmt.Sprintf("%d",repo.ExtraInfor.Size) + "KB")
	return extraStyle.Width(totalWidth).Render(header + size + watchers + openissues + SubscribersCount )

}
