package ui

import (
	"fmt"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderExtraInfor(totalWidth int, th theme.Theme) string {
	if totalWidth <= 0 {
		totalWidth = 44
	}
	repo := m.getCursorRepo()

	header := renderHeader(totalWidth, "Project Metrics", th)
	commits := renderEachField(totalWidth, "Total Commits:", fmt.Sprintf("%d", repo.ExtraInfor.TotalCommits))
	prs := renderEachField(totalWidth, "Pull Requests:", fmt.Sprintf("%s", repo.ExtraInfor.PullRequests))
	issues := renderEachField(totalWidth, "Open Issues:", fmt.Sprintf("%s", repo.ExtraInfor.Issues))
	contributors := renderEachField(totalWidth, "Contributors:", fmt.Sprintf("%d", repo.ExtraInfor.Contributors))

	extraStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(th.Border).
		Padding(0, 1)

	return extraStyle.Width(totalWidth).Render(header + commits + prs + issues + contributors)
}
