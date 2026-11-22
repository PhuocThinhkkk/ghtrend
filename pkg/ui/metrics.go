package ui

import (
	"fmt"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderExtraInfor(totalWidth, height int, th theme.Theme) string {
	if totalWidth <= 0 {
		totalWidth = 44
	}
	repo := m.getCursorRepo()

	lineWidth := max(10, totalWidth-4)
	header := renderHeader(lineWidth, "Project Metrics", th)
	commits := renderEachField(lineWidth, "Total Commits:", fmt.Sprintf("%d", repo.ExtraInfor.TotalCommits))
	prs := renderEachField(lineWidth, "Pull Requests:", fmt.Sprintf("%s", repo.ExtraInfor.PullRequests))
	issues := renderEachField(lineWidth, "Open Issues:", fmt.Sprintf("%s", repo.ExtraInfor.Issues))
	contributors := renderEachField(lineWidth, "Contributors:", fmt.Sprintf("%d", repo.ExtraInfor.Contributors))

	extraStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(th.Border).
		Padding(0, 1).
		Width(max(10, totalWidth-2)).
		MaxWidth(totalWidth)

	if height > 0 {
		extraStyle = extraStyle.Height(max(1, height-2)).MaxHeight(height)
	}

	content := strings.TrimRight(header+commits+prs+issues+contributors, "\n")
	return extraStyle.Render(content)
}
