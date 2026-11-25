package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wsIssuesLoadedMsg struct {
	seq     uint64
	reqSeq  uint64
	filter  string
	page    int
	issues  []ghclient.Issue
	hasNext bool
	err     error
}

type wsIssueTimelineLoadedMsg struct {
	seq      uint64
	number   int
	comments []ghclient.TimelineComment
	err      error
}

type wsIssueStateUpdatedMsg struct {
	seq    uint64
	number int
	issue  ghclient.Issue
	err    error
}

func wsFetchIssuesCmd(svc WorkspaceService, seq, reqSeq uint64, owner, repo, state string, page int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		issues, hasNext, err := svc.FetchIssues(ctx, owner, repo, state, page)
		return wsIssuesLoadedMsg{seq: seq, reqSeq: reqSeq, filter: state, page: page, issues: issues, hasNext: hasNext, err: err}
	}
}

func wsFetchIssueTimelineCmd(svc WorkspaceService, seq uint64, owner, repo string, number int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		comments, err := svc.FetchIssueTimeline(ctx, owner, repo, number)
		return wsIssueTimelineLoadedMsg{seq: seq, number: number, comments: comments, err: err}
	}
}

func wsUpdateIssueStateCmd(svc WorkspaceService, seq uint64, owner, repo string, number int, state string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		issue, err := svc.UpdateIssueState(ctx, owner, repo, number, state)
		return wsIssueStateUpdatedMsg{seq: seq, number: number, issue: issue, err: err}
	}
}

func (m *Model) ensureWorkspaceIssuesLoaded() tea.Cmd {
	if m.workspace.issuesLoaded || m.workspace.issuesLoading {
		return nil
	}
	if m.workspace.issueFilter == "" {
		m.workspace.issueFilter = "open"
	}
	if m.workspace.issuePage < 1 {
		m.workspace.issuePage = 1
	}
	m.workspace.issueSeq++
	m.workspace.issuesLoading = true
	m.workspace.issuesErr = nil
	return wsFetchIssuesCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.issueSeq, m.workspace.owner, m.workspace.repo, m.workspace.issueFilter, m.workspace.issuePage)
}

func (m *Model) reloadWorkspaceIssues(page int) tea.Cmd {
	if page < 1 {
		page = 1
	}
	if m.workspace.issueFilter == "" {
		m.workspace.issueFilter = "open"
	}
	m.workspace.issueSeq++
	m.workspace.issuePage = page
	m.workspace.issuesLoading = true
	m.workspace.issuesErr = nil
	m.workspace.issueCursor = 0
	m.workspace.issueOffset = 0
	return wsFetchIssuesCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.issueSeq, m.workspace.owner, m.workspace.repo, m.workspace.issueFilter, page)
}

func (m *Model) openIssueDetail(issue ghclient.Issue) tea.Cmd {
	cp := issue
	m.workspace.issueDetail = &cp
	m.workspace.issueTimeline = nil
	m.workspace.issueTimelineErr = nil
	m.workspace.issueTimelineLoading = true
	m.workspace.issueViewport.GotoTop()
	m.refreshIssueDetailContent()
	return wsFetchIssueTimelineCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, issue.Number)
}

func (m *Model) refreshIssueDetailContent() {
	if m.workspace.issueDetail == nil {
		return
	}
	iss := m.workspace.issueDetail
	w := max(24, m.workspace.issueViewport.Width)

	statusBadge := "🟢 OPEN"
	if strings.EqualFold(iss.State, "closed") {
		statusBadge = "🟣 CLOSED"
	}
	var metaParts []string
	metaParts = append(metaParts, statusBadge, fmt.Sprintf("by @%s", iss.Author))
	if len(iss.Assignees) > 0 {
		metaParts = append(metaParts, "Assignees: @"+strings.Join(iss.Assignees, ", @"))
	}
	if len(iss.Labels) > 0 {
		names := make([]string, 0, len(iss.Labels))
		for _, lbl := range iss.Labels {
			names = append(names, "`"+lbl.Name+"`")
		}
		metaParts = append(metaParts, "Labels: "+strings.Join(names, " "))
	}

	title := fmt.Sprintf("#%d %s", iss.Number, iss.Title)
	rendered := renderTimelineMarkdown(title, strings.Join(metaParts, " · "), iss.Body, m.workspace.issueTimeline, m.workspace.issueTimelineLoading, m.workspace.issueTimelineErr, m.theme, w)
	m.workspace.issueViewport.SetContent(rendered)
}

func (m *Model) toggleIssueState(iss ghclient.Issue) tea.Cmd {
	if m.workspace.issueStateUpdating {
		return nil
	}
	nextState := "closed"
	if strings.EqualFold(iss.State, "closed") {
		nextState = "open"
	}
	m.workspace.issueStateUpdating = true
	return wsUpdateIssueStateCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, iss.Number, nextState)
}

func (m *Model) updateWorkspaceIssues(msg tea.KeyMsg) tea.Cmd {
	if m.workspace.issueDetail != nil {
		switch msg.String() {
		case "esc", "backspace":
			m.workspace.issueDetail = nil
			m.workspace.issueTimeline = nil
			m.workspace.issueTimelineErr = nil
			m.workspace.issueTimelineLoading = false
			return nil
		case "c":
			return m.openCommentModal("issue", m.workspace.issueDetail.Number)
		case "X":
			return m.toggleIssueState(*m.workspace.issueDetail)
		case "r":
			m.workspace.issueTimelineLoading = true
			m.workspace.issueTimelineErr = nil
			m.refreshIssueDetailContent()
			return wsFetchIssueTimelineCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, m.workspace.issueDetail.Number)
		case "up", "k":
			m.workspace.issueViewport.ScrollUp(1)
			return nil
		case "down", "j":
			m.workspace.issueViewport.ScrollDown(1)
			return nil
		case "pgup", "u":
			m.workspace.issueViewport.HalfPageUp()
			return nil
		case "pgdown", "d":
			m.workspace.issueViewport.HalfPageDown()
			return nil
		case "g":
			m.workspace.issueViewport.GotoTop()
			return nil
		case "G":
			m.workspace.issueViewport.GotoBottom()
			return nil
		}
		var cmd tea.Cmd
		m.workspace.issueViewport, cmd = m.workspace.issueViewport.Update(msg)
		return cmd
	}

	visRows := m.workspaceListVisibleRows()
	switch msg.String() {
	case "f":
		m.workspace.issueFilter = nextIssuePRStateFilter(m.workspace.issueFilter)
		return m.reloadWorkspaceIssues(1)
	case "r":
		return m.reloadWorkspaceIssues(m.workspace.issuePage)
	case "]", "n":
		if m.workspace.issueHasNext {
			return m.reloadWorkspaceIssues(m.workspace.issuePage + 1)
		}
		return nil
	case "[", "p":
		if m.workspace.issuePage > 1 {
			return m.reloadWorkspaceIssues(m.workspace.issuePage - 1)
		}
		return nil
	case "up", "k":
		if m.workspace.issueCursor > 0 {
			m.workspace.issueCursor--
			m.workspace.clampIssueCursor(visRows)
		}
		return nil
	case "down", "j":
		if m.workspace.issueCursor < len(m.workspace.issues)-1 {
			m.workspace.issueCursor++
			m.workspace.clampIssueCursor(visRows)
		}
		return nil
	case "g":
		m.workspace.issueCursor = 0
		m.workspace.issueOffset = 0
		return nil
	case "G":
		if len(m.workspace.issues) > 0 {
			m.workspace.issueCursor = len(m.workspace.issues) - 1
			m.workspace.clampIssueCursor(visRows)
		}
		return nil
	case "enter", "l":
		if m.workspace.issueCursor >= 0 && m.workspace.issueCursor < len(m.workspace.issues) {
			return m.openIssueDetail(m.workspace.issues[m.workspace.issueCursor])
		}
		return nil
	case "c":
		if m.workspace.issueCursor >= 0 && m.workspace.issueCursor < len(m.workspace.issues) {
			return m.openCommentModal("issue", m.workspace.issues[m.workspace.issueCursor].Number)
		}
		return nil
	case "X":
		if m.workspace.issueCursor >= 0 && m.workspace.issueCursor < len(m.workspace.issues) {
			return m.toggleIssueState(m.workspace.issues[m.workspace.issueCursor])
		}
		return nil
	}
	return nil
}

func (w *workspaceModel) clampIssueCursor(visRows int) {
	if len(w.issues) == 0 {
		w.issueCursor = 0
		w.issueOffset = 0
		return
	}
	if w.issueCursor < 0 {
		w.issueCursor = 0
	}
	if w.issueCursor >= len(w.issues) {
		w.issueCursor = len(w.issues) - 1
	}
	if visRows < 1 {
		visRows = 1
	}
	if w.issueCursor < w.issueOffset {
		w.issueOffset = w.issueCursor
	}
	if w.issueCursor >= w.issueOffset+visRows {
		w.issueOffset = w.issueCursor - visRows + 1
	}
	maxOff := max(0, len(w.issues)-visRows)
	if w.issueOffset > maxOff {
		w.issueOffset = maxOff
	}
}

func (m Model) renderWorkspaceIssuesTab(width, availH int) string {
	if m.workspace.issueDetail != nil {
		iss := m.workspace.issueDetail
		title := fmt.Sprintf("Issue #%d: %s  (Esc back • c comment • X close/reopen)", iss.Number, TruncateDisplayWidth(iss.Title, max(16, width-52), "…"))
		content := m.workspace.issueViewport.View()
		return WrapInTitledBorder(content, width, availH, title, true, m.theme)
	}

	innerW := max(16, width-4)
	filterBar := TruncateDisplayWidth(
		renderFilterPills(m.theme, m.workspace.issueFilter)+
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render(fmt.Sprintf("   Page %d   (f filter • Enter open • c comment • X close/reopen)", max(1, m.workspace.issuePage))),
		innerW,
		"…",
	)

	var lines []string
	lines = append(lines, filterBar)
	lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", innerW)))

	if m.workspace.issuesLoading {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Loading issues..."))
	} else if m.workspace.issuesErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("⚠ "+m.workspace.issuesErr.Error()))
	} else if len(m.workspace.issues) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No issues found for filter '"+m.workspace.issueFilter+"'."))
	} else {
		visRows := m.workspaceListVisibleRows()
		end := min(len(m.workspace.issues), m.workspace.issueOffset+visRows)
		for i := m.workspace.issueOffset; i < end; i++ {
			iss := m.workspace.issues[i]
			selected := i == m.workspace.issueCursor
			lines = append(lines, m.renderIssueRow(iss, innerW, selected))
		}
	}

	title := fmt.Sprintf("Issues (%s)", strings.ToUpper(m.workspace.issueFilter))
	return WrapInTitledBorder(strings.Join(lines, "\n"), width, availH, title, true, m.theme)
}

func (m Model) renderIssueRow(iss ghclient.Issue, width int, selected bool) string {
	icon := "🟢"
	if strings.EqualFold(iss.State, "closed") {
		icon = "🟣"
	}

	assigneeText := ""
	if len(iss.Assignees) > 0 {
		assigneeText = " →@" + iss.Assignees[0]
	}
	right := fmt.Sprintf("@%s%s 💬%d", iss.Author, assigneeText, iss.CommentCount)
	rightW := lipgloss.Width(right)

	labelsStr := ""
	if !selected && len(iss.Labels) > 0 && width >= 70 {
		labelsStr = " " + renderLabelBadges(iss.Labels, max(10, width/4))
	}
	labelsW := lipgloss.Width(labelsStr)

	leftMax := max(12, width-rightW-labelsW-3)
	left := TruncateDisplayWidth(fmt.Sprintf("%s #%d %s", icon, iss.Number, iss.Title), leftMax, "…")
	gap := max(1, width-lipgloss.Width(left)-labelsW-rightW)
	row := left + labelsStr + strings.Repeat(" ", gap) + right

	if selected {
		plain := TruncateDisplayWidth(row, width, "…")
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Background).
			Background(m.theme.Primary).
			Width(width).
			Render(plain)
	}
	return lipgloss.NewStyle().Foreground(m.theme.Text).Width(width).Render(TruncateDisplayWidth(row, width, "…"))
}
