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
	commits := renderEachField(totalWidth, "Total Commits: ", fmt.Sprintf("%d",repo.ExtraInfor.TotalCommits))
	prs := renderEachField(totalWidth, "Pull Requests:", fmt.Sprintf("%s", repo.ExtraInfor.PullRequests))
	issues := renderEachField(totalWidth, "Open Issues:", fmt.Sprintf("%s", repo.ExtraInfor.Issues))
	contributors := renderEachField(totalWidth, "Contributors Count: ", fmt.Sprintf("%d", repo.ExtraInfor.Contributors))
	return extraStyle.Width(totalWidth).Render(header + commits + prs + issues + contributors )

}
