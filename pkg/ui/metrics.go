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
	commits := renderEachFieldWithTheme(lineWidth, "Total Commits:", fmt.Sprintf("%d", repo.ExtraInfor.TotalCommits), th)

	prsVal := strings.TrimSpace(repo.ExtraInfor.PullRequests)
	if prsVal == "" {
		prsVal = "0"
	}
	prs := renderEachFieldWithTheme(lineWidth, "Pull Requests:", prsVal, th)

	issuesVal := strings.TrimSpace(repo.ExtraInfor.Issues)
	if issuesVal == "" {
		issuesVal = "0"
	}
	issues := renderEachFieldWithTheme(lineWidth, "Open Issues:", issuesVal, th)

	contribVal := repo.ExtraInfor.Contributors
	if contribVal <= 0 {
		contribVal = 1
	}
	contributors := renderEachFieldWithTheme(lineWidth, "Contributors:", fmt.Sprintf("%d", contribVal), th)

	content := strings.TrimRight(commits+prs+issues+contributors, "\n")
	return WrapInTitledBorder(content, totalWidth, height, "Project Metrics", false, th)
}

// renderMetricsLanguagesAndCommits renders a single, unified right-hand panel
// containing Project Metrics, Languages Breakdown, and Recent Commits
// connected by seamless inner dividers (├─ ... ─┤).
func (m Model) renderMetricsLanguagesAndCommits(totalWidth, totalHeight int, th theme.Theme) string {
	if totalWidth <= 0 {
		totalWidth = 36
	}
	if totalHeight <= 0 {
		totalHeight = 16
	}

	repo := m.getCursorRepo()
	lineWidth := max(10, totalWidth-4)
	innerW := max(10, totalWidth-2)

	borderColor := th.Border
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	leftB := borderStyle.Render("│")
	rightB := borderStyle.Render("│")

	wrapLine := func(content string) string {
		w := lipgloss.Width(content)
		if w > lineWidth {
			content = lipgloss.NewStyle().MaxWidth(lineWidth).Render(content)
			w = lipgloss.Width(content)
		}
		pad := lineWidth - w
		if pad < 0 {
			pad = 0
		}
		return leftB + " " + content + strings.Repeat(" ", pad) + " " + rightB
	}

	// 1. Top border
	topBorder := RenderTitledBorderTop(totalWidth, "Project Metrics", false, th)

	// 2. Metrics Rows
	commitsVal := fmt.Sprintf("%d", repo.ExtraInfor.TotalCommits)
	prsVal := strings.TrimSpace(repo.ExtraInfor.PullRequests)
	if prsVal == "" {
		prsVal = "0"
	}
	issuesVal := strings.TrimSpace(repo.ExtraInfor.Issues)
	if issuesVal == "" {
		issuesVal = "0"
	}
	contribVal := repo.ExtraInfor.Contributors
	if contribVal <= 0 {
		contribVal = 1
	}

	metricsLines := []string{
		strings.TrimRight(renderEachFieldWithTheme(lineWidth, "Total Commits:", commitsVal, th), "\n"),
		strings.TrimRight(renderEachFieldWithTheme(lineWidth, "Pull Requests:", prsVal, th), "\n"),
		strings.TrimRight(renderEachFieldWithTheme(lineWidth, "Open Issues:", issuesVal, th), "\n"),
		strings.TrimRight(renderEachFieldWithTheme(lineWidth, "Contributors:", fmt.Sprintf("%d", contribVal), th), "\n"),
	}

	// 3. Languages Divider and Content
	langDivider := RenderInnerDivider(totalWidth, "Languages Breakdown", false, th)
	pairs := getSortedLangPairs(repo)
	bar := renderStackedLanguageBar(pairs, lineWidth, th)
	dots := renderLanguageDots(pairs, lineWidth, 1, th)

	// 4. Recent Commits Divider and Content
	commitsDivider := RenderInnerDivider(totalWidth, "Recent Commits", false, th)

	var commitRows []string
	commits := repo.RecentCommits
	if len(commits) == 0 {
		commitRows = append(commitRows, lipgloss.NewStyle().Foreground(th.Muted).Render("· No recent commits"))
	} else {
		for _, c := range commits {
			hashStyle := lipgloss.NewStyle().Foreground(th.Accent).Bold(true).Render(c.Hash)
			msg := c.Message
			maxMsgW := lineWidth - lipgloss.Width(hashStyle) - 1
			if maxMsgW > 3 && lipgloss.Width(msg) > maxMsgW {
				runes := []rune(msg)
				if len(runes) > maxMsgW {
					msg = string(runes[:maxMsgW-2]) + ".."
				}
			}
			msgStyle := lipgloss.NewStyle().Foreground(th.Text).Render(msg)
			commitRows = append(commitRows, hashStyle+" "+msgStyle)
		}
	}

	// 5. Budget lines to fit exactly within totalHeight
	// Overhead lines: top(1) + divider1(1) + divider2(1) + bottom(1) = 4 lines overhead.
	// Total available content lines = totalHeight - 4.
	availLines := totalHeight - 4
	if availLines < 3 {
		availLines = 3
	}

	// Line allocation:
	// Metrics gets up to 4 lines
	// Languages gets 2 lines (bar + dots)
	// Commits gets remainder
	numMetrics := min(4, max(1, availLines/3))
	numLang := 2
	if availLines < 6 {
		numLang = 1
	}
	numCommits := availLines - numMetrics - numLang
	if numCommits < 1 {
		numCommits = 1
	}

	var allBodyLines []string
	// Add metrics
	for i := 0; i < numMetrics && i < len(metricsLines); i++ {
		allBodyLines = append(allBodyLines, wrapLine(metricsLines[i]))
	}

	// Add languages divider
	allBodyLines = append(allBodyLines, langDivider)
	allBodyLines = append(allBodyLines, wrapLine(bar))
	if numLang >= 2 {
		allBodyLines = append(allBodyLines, wrapLine(dots))
	}

	// Add commits divider
	allBodyLines = append(allBodyLines, commitsDivider)
	for i := 0; i < numCommits; i++ {
		if i < len(commitRows) {
			allBodyLines = append(allBodyLines, wrapLine(commitRows[i]))
		} else {
			allBodyLines = append(allBodyLines, wrapLine(""))
		}
	}

	// Truncate or pad to exactly totalHeight - 2
	targetLines := totalHeight - 2
	for len(allBodyLines) < targetLines {
		allBodyLines = append(allBodyLines, wrapLine(""))
	}
	if len(allBodyLines) > targetLines {
		allBodyLines = allBodyLines[:targetLines]
	}

	bottomBorder := RenderTitledBorderBottom(totalWidth, false, th)
	res := topBorder + "\n" + strings.Join(allBodyLines, "\n") + "\n" + bottomBorder

	_ = innerW
	return res
}
