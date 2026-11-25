package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wsPRsLoadedMsg struct {
	seq     uint64
	reqSeq  uint64
	filter  string
	page    int
	prs     []ghclient.PullRequest
	hasNext bool
	err     error
}

type wsPRTimelineLoadedMsg struct {
	seq      uint64
	number   int
	comments []ghclient.TimelineComment
	err      error
}

type wsCommentPostedMsg struct {
	seq     uint64
	reqSeq  uint64
	kind    string // "pr" or "issue"
	number  int
	comment ghclient.TimelineComment
	err     error
}

func wsFetchPRsCmd(svc WorkspaceService, seq, reqSeq uint64, owner, repo, state string, page int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		prs, hasNext, err := svc.FetchPRs(ctx, owner, repo, state, page)
		return wsPRsLoadedMsg{seq: seq, reqSeq: reqSeq, filter: state, page: page, prs: prs, hasNext: hasNext, err: err}
	}
}

func wsFetchPRTimelineCmd(svc WorkspaceService, seq uint64, owner, repo string, number int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		comments, err := svc.FetchPRTimeline(ctx, owner, repo, number)
		return wsPRTimelineLoadedMsg{seq: seq, number: number, comments: comments, err: err}
	}
}

func wsCreateCommentCmd(svc WorkspaceService, seq, reqSeq uint64, kind, owner, repo string, number int, body string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		comment, err := svc.CreateComment(ctx, owner, repo, number, body)
		return wsCommentPostedMsg{seq: seq, reqSeq: reqSeq, kind: kind, number: number, comment: comment, err: err}
	}
}

func (m *Model) ensureWorkspacePRsLoaded() tea.Cmd {
	if m.workspace.prsLoaded || m.workspace.prsLoading {
		return nil
	}
	if m.workspace.prFilter == "" {
		m.workspace.prFilter = "open"
	}
	if m.workspace.prPage < 1 {
		m.workspace.prPage = 1
	}
	m.workspace.prSeq++
	m.workspace.prsLoading = true
	m.workspace.prsErr = nil
	return wsFetchPRsCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.prSeq, m.workspace.owner, m.workspace.repo, m.workspace.prFilter, m.workspace.prPage)
}

func (m *Model) reloadWorkspacePRs(page int) tea.Cmd {
	if page < 1 {
		page = 1
	}
	if m.workspace.prFilter == "" {
		m.workspace.prFilter = "open"
	}
	m.workspace.prSeq++
	m.workspace.prPage = page
	m.workspace.prsLoading = true
	m.workspace.prsErr = nil
	m.workspace.prCursor = 0
	m.workspace.prOffset = 0
	return wsFetchPRsCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.prSeq, m.workspace.owner, m.workspace.repo, m.workspace.prFilter, page)
}

func (m *Model) openPRDetail(pr ghclient.PullRequest) tea.Cmd {
	cp := pr
	m.workspace.prDetail = &cp
	m.workspace.prTimeline = nil
	m.workspace.prTimelineErr = nil
	m.workspace.prTimelineLoading = true
	m.workspace.prViewport.GotoTop()
	m.refreshPRDetailContent()
	return wsFetchPRTimelineCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, pr.Number)
}

func (m *Model) refreshPRDetailContent() {
	if m.workspace.prDetail == nil {
		return
	}
	pr := m.workspace.prDetail
	w := max(24, m.workspace.prViewport.Width)

	statusBadge := "🟢 OPEN"
	if pr.Merged {
		statusBadge = "🟣 MERGED"
	} else if strings.EqualFold(pr.State, "closed") {
		statusBadge = "🔴 CLOSED"
	} else if pr.Draft {
		statusBadge = "⚪ DRAFT"
	}

	var metaParts []string
	metaParts = append(metaParts,
		statusBadge,
		fmt.Sprintf("by @%s", pr.Author),
		fmt.Sprintf("%s → %s", pr.HeadRef, pr.BaseRef),
	)
	if pr.ReviewDecision != "" {
		metaParts = append(metaParts, "Review: "+pr.ReviewDecision)
	}
	if pr.CheckState != "" {
		metaParts = append(metaParts, "Checks: "+pr.CheckState)
	}
	if pr.Additions > 0 || pr.Deletions > 0 || pr.ChangedFiles > 0 {
		metaParts = append(metaParts, fmt.Sprintf("+%d/-%d (%d files)", pr.Additions, pr.Deletions, pr.ChangedFiles))
	}
	if len(pr.Labels) > 0 {
		names := make([]string, 0, len(pr.Labels))
		for _, lbl := range pr.Labels {
			names = append(names, "`"+lbl.Name+"`")
		}
		metaParts = append(metaParts, "Labels: "+strings.Join(names, " "))
	}

	title := fmt.Sprintf("#%d %s", pr.Number, pr.Title)
	rendered := renderTimelineMarkdown(title, strings.Join(metaParts, " · "), pr.Body, m.workspace.prTimeline, m.workspace.prTimelineLoading, m.workspace.prTimelineErr, m.theme, w)
	m.workspace.prViewport.SetContent(rendered)
}

func (m *Model) updateWorkspacePRs(msg tea.KeyMsg) tea.Cmd {
	if m.workspace.prDetail != nil {
		switch msg.String() {
		case "esc", "backspace":
			m.workspace.prDetail = nil
			m.workspace.prTimeline = nil
			m.workspace.prTimelineErr = nil
			m.workspace.prTimelineLoading = false
			return nil
		case "c":
			return m.openCommentModal("pr", m.workspace.prDetail.Number)
		case "r":
			m.workspace.prTimelineLoading = true
			m.workspace.prTimelineErr = nil
			m.refreshPRDetailContent()
			return wsFetchPRTimelineCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.owner, m.workspace.repo, m.workspace.prDetail.Number)
		case "up", "k":
			m.workspace.prViewport.ScrollUp(1)
			return nil
		case "down", "j":
			m.workspace.prViewport.ScrollDown(1)
			return nil
		case "pgup", "u":
			m.workspace.prViewport.HalfPageUp()
			return nil
		case "pgdown", "d":
			m.workspace.prViewport.HalfPageDown()
			return nil
		case "g":
			m.workspace.prViewport.GotoTop()
			return nil
		case "G":
			m.workspace.prViewport.GotoBottom()
			return nil
		}
		var cmd tea.Cmd
		m.workspace.prViewport, cmd = m.workspace.prViewport.Update(msg)
		return cmd
	}

	visRows := m.workspaceListVisibleRows()
	switch msg.String() {
	case "f":
		m.workspace.prFilter = nextIssuePRStateFilter(m.workspace.prFilter)
		return m.reloadWorkspacePRs(1)
	case "r":
		return m.reloadWorkspacePRs(m.workspace.prPage)
	case "]", "n":
		if m.workspace.prHasNext {
			return m.reloadWorkspacePRs(m.workspace.prPage + 1)
		}
		return nil
	case "[", "p":
		if m.workspace.prPage > 1 {
			return m.reloadWorkspacePRs(m.workspace.prPage - 1)
		}
		return nil
	case "up", "k":
		if m.workspace.prCursor > 0 {
			m.workspace.prCursor--
			m.workspace.clampPRCursor(visRows)
		}
		return nil
	case "down", "j":
		if m.workspace.prCursor < len(m.workspace.prs)-1 {
			m.workspace.prCursor++
			m.workspace.clampPRCursor(visRows)
		}
		return nil
	case "g":
		m.workspace.prCursor = 0
		m.workspace.prOffset = 0
		return nil
	case "G":
		if len(m.workspace.prs) > 0 {
			m.workspace.prCursor = len(m.workspace.prs) - 1
			m.workspace.clampPRCursor(visRows)
		}
		return nil
	case "enter", "l":
		if m.workspace.prCursor >= 0 && m.workspace.prCursor < len(m.workspace.prs) {
			return m.openPRDetail(m.workspace.prs[m.workspace.prCursor])
		}
		return nil
	case "c":
		if m.workspace.prCursor >= 0 && m.workspace.prCursor < len(m.workspace.prs) {
			return m.openCommentModal("pr", m.workspace.prs[m.workspace.prCursor].Number)
		}
		return nil
	}
	return nil
}

func (w *workspaceModel) clampPRCursor(visRows int) {
	if len(w.prs) == 0 {
		w.prCursor = 0
		w.prOffset = 0
		return
	}
	if w.prCursor < 0 {
		w.prCursor = 0
	}
	if w.prCursor >= len(w.prs) {
		w.prCursor = len(w.prs) - 1
	}
	if visRows < 1 {
		visRows = 1
	}
	if w.prCursor < w.prOffset {
		w.prOffset = w.prCursor
	}
	if w.prCursor >= w.prOffset+visRows {
		w.prOffset = w.prCursor - visRows + 1
	}
	maxOff := max(0, len(w.prs)-visRows)
	if w.prOffset > maxOff {
		w.prOffset = maxOff
	}
}

func (m Model) renderWorkspacePRsTab(width, availH int) string {
	if m.workspace.prDetail != nil {
		pr := m.workspace.prDetail
		title := fmt.Sprintf("PR #%d: %s  (Esc back • c comment • r refresh)", pr.Number, TruncateDisplayWidth(pr.Title, max(16, width-48), "…"))
		content := m.workspace.prViewport.View()
		return WrapInTitledBorder(content, width, availH, title, true, m.theme)
	}

	innerW := max(16, width-4)
	filterBar := TruncateDisplayWidth(
		renderFilterPills(m.theme, m.workspace.prFilter)+
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render(fmt.Sprintf("   Page %d   (f filter • Enter open • c comment • [/] page)", max(1, m.workspace.prPage))),
		innerW,
		"…",
	)

	var lines []string
	lines = append(lines, filterBar)
	lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", innerW)))

	if m.workspace.prsLoading {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Loading pull requests..."))
	} else if m.workspace.prsErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("⚠ "+m.workspace.prsErr.Error()))
	} else if len(m.workspace.prs) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No pull requests found for filter '"+m.workspace.prFilter+"'."))
	} else {
		visRows := m.workspaceListVisibleRows()
		end := min(len(m.workspace.prs), m.workspace.prOffset+visRows)
		for i := m.workspace.prOffset; i < end; i++ {
			pr := m.workspace.prs[i]
			selected := i == m.workspace.prCursor
			lines = append(lines, m.renderPRRow(pr, innerW, selected))
		}
	}

	title := fmt.Sprintf("Pull Requests (%s)", strings.ToUpper(m.workspace.prFilter))
	return WrapInTitledBorder(strings.Join(lines, "\n"), width, availH, title, true, m.theme)
}

func (m Model) renderPRRow(pr ghclient.PullRequest, width int, selected bool) string {
	icon := "🟢"
	if pr.Merged {
		icon = "🟣"
	} else if strings.EqualFold(pr.State, "closed") {
		icon = "🔴"
	} else if pr.Draft {
		icon = "⚪"
	}

	reviewBadge := ""
	switch strings.ToUpper(pr.ReviewDecision) {
	case "APPROVED":
		reviewBadge = " ✓"
	case "CHANGES_REQUESTED":
		reviewBadge = " !"
	case "REVIEW_REQUIRED":
		reviewBadge = " ?"
	}

	ciBadge := ""
	switch strings.ToUpper(pr.CheckState) {
	case "SUCCESS":
		ciBadge = " ●"
	case "FAILURE", "ERROR":
		ciBadge = " ✗"
	case "PENDING", "EXPECTED":
		ciBadge = " ◷"
	}

	branchInfo := ""
	if pr.HeadRef != "" && pr.BaseRef != "" {
		branchInfo = fmt.Sprintf(" [%s→%s]", pr.HeadRef, pr.BaseRef)
	}
	right := fmt.Sprintf("@%s%s%s 💬%d", pr.Author, reviewBadge, ciBadge, pr.CommentCount)
	rightW := lipgloss.Width(right)
	leftMax := max(12, width-rightW-3)
	left := TruncateDisplayWidth(fmt.Sprintf("%s #%d %s%s", icon, pr.Number, pr.Title, branchInfo), leftMax, "…")
	gap := max(1, width-lipgloss.Width(left)-rightW)
	row := left + strings.Repeat(" ", gap) + right
	row = TruncateDisplayWidth(row, width, "…")

	if selected {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Background).
			Background(m.theme.Primary).
			Width(width).
			Render(row)
	}
	return lipgloss.NewStyle().Foreground(m.theme.Text).Width(width).Render(row)
}

// --- Shared helpers for PRs & Issues ---

func nextIssuePRStateFilter(curr string) string {
	switch strings.ToLower(curr) {
	case "open":
		return "closed"
	case "closed":
		return "all"
	default:
		return "open"
	}
}

func renderFilterPills(th theme.Theme, active string) string {
	active = strings.ToLower(active)
	if active == "" {
		active = "open"
	}
	states := []string{"open", "closed", "all"}
	parts := make([]string, 0, len(states))
	for _, st := range states {
		label := strings.ToUpper(st[:1]) + st[1:]
		if st == active {
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).
				Foreground(th.Background).
				Background(th.Secondary).
				Padding(0, 1).
				Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(th.Muted).
				Background(th.Surface).
				Padding(0, 1).
				Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func renderLabelBadges(labels []ghclient.Label, maxW int) string {
	if len(labels) == 0 || maxW <= 0 {
		return ""
	}
	var badges []string
	used := 0
	for _, lbl := range labels {
		hex := strings.TrimPrefix(strings.TrimSpace(lbl.Color), "#")
		bg := lipgloss.Color("#3b4261")
		fg := lipgloss.Color("#c0caf5")
		if len(hex) == 6 {
			bg = lipgloss.Color("#" + hex)
			if isBrightHex(hex) {
				fg = lipgloss.Color("#1a1b26")
			} else {
				fg = lipgloss.Color("#ffffff")
			}
		}
		badge := lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 1).Render(lbl.Name)
		bw := lipgloss.Width(badge)
		if used > 0 && used+1+bw > maxW {
			break
		}
		badges = append(badges, badge)
		used += bw + 1
	}
	return strings.Join(badges, " ")
}

func isBrightHex(hex string) bool {
	if len(hex) != 6 {
		return false
	}
	r, err1 := strconv.ParseInt(hex[0:2], 16, 64)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 64)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	lum := (299*r + 587*g + 114*b) / 1000
	return lum > 140
}

func renderTimelineMarkdown(title, meta, body string, comments []ghclient.TimelineComment, loading bool, err error, th theme.Theme, width int) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	if meta != "" {
		sb.WriteString(meta)
		sb.WriteString("\n\n---\n\n")
	}
	if strings.TrimSpace(body) != "" {
		sb.WriteString(body)
	} else {
		sb.WriteString("_No description provided._")
	}
	sb.WriteString("\n\n---\n\n## Timeline\n\n")
	if loading {
		sb.WriteString("_Loading timeline comments..._\n")
	} else if err != nil {
		sb.WriteString("⚠ Failed to load timeline: " + err.Error() + "\n")
	} else if len(comments) == 0 {
		sb.WriteString("_No comments yet. Press `c` to add a comment._\n")
	} else {
		for _, c := range comments {
			ts := c.CreatedAt.Format("2006-01-02 15:04")
			if c.CreatedAt.IsZero() {
				ts = "just now"
			}
			header := fmt.Sprintf("### @%s · %s", c.Author, ts)
			if c.Kind == "review" && c.State != "" {
				header += fmt.Sprintf(" · **%s**", c.State)
			} else if c.Kind == "review_comment" && c.Path != "" {
				header += fmt.Sprintf(" · `%s`", c.Path)
			}
			sb.WriteString(header)
			sb.WriteString("\n\n")
			if strings.TrimSpace(c.Body) != "" {
				sb.WriteString(c.Body)
			} else if c.Kind == "review" {
				sb.WriteString("_Submitted review (" + c.State + ")._")
			}
			sb.WriteString("\n\n---\n\n")
		}
	}
	return RenderFullMarkdown(sb.String(), th, width)
}

func (m *Model) openCommentModal(kind string, number int) tea.Cmd {
	ta := textarea.New()
	ta.Placeholder = "Write your GitHub comment (Markdown supported)..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 8192
	w := max(36, min(76, m.width-12))
	h := max(5, min(10, m.height-12))
	ta.SetWidth(w)
	ta.SetHeight(h)
	ta.Focus()

	m.workspace.commentSeq++
	m.workspace.showCommentModal = true
	m.workspace.commentTargetKind = kind
	m.workspace.commentTargetNumber = number
	m.workspace.commentInput = ta
	m.workspace.commentSubmitting = false
	m.workspace.commentErr = nil
	return textarea.Blink
}

func (m *Model) updateWorkspaceCommentModal(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.workspace.commentSeq++
		m.workspace.showCommentModal = false
		m.workspace.commentSubmitting = false
		m.workspace.commentErr = nil
		m.workspace.commentInput.Blur()
		return nil
	case "ctrl+s":
		if m.workspace.commentSubmitting {
			return nil
		}
		body := strings.TrimSpace(m.workspace.commentInput.Value())
		if body == "" {
			m.workspace.commentErr = fmt.Errorf("comment cannot be empty")
			return nil
		}
		m.workspace.commentSubmitting = true
		m.workspace.commentErr = nil
		return wsCreateCommentCmd(
			m.activeWorkspaceService(),
			m.workspace.seq,
			m.workspace.commentSeq,
			m.workspace.commentTargetKind,
			m.workspace.owner,
			m.workspace.repo,
			m.workspace.commentTargetNumber,
			body,
		)
	default:
		var cmd tea.Cmd
		m.workspace.commentInput, cmd = m.workspace.commentInput.Update(msg)
		return cmd
	}
}

func (m Model) renderWorkspaceCommentModal(width, height int) string {
	modalW := max(42, min(82, width-6))
	kindLabel := "Issue"
	if m.workspace.commentTargetKind == "pr" {
		kindLabel = "PR"
	}
	title := fmt.Sprintf("Comment on %s #%d", kindLabel, m.workspace.commentTargetNumber)

	var parts []string
	parts = append(parts, m.workspace.commentInput.View())
	if m.workspace.commentSubmitting {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Secondary).Render("Posting comment to GitHub..."))
	} else if m.workspace.commentErr != nil {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("⚠ "+m.workspace.commentErr.Error()))
	} else {
		parts = append(parts, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Ctrl+S submit  •  Esc cancel"))
	}

	body := strings.Join(parts, "\n\n")
	modalH := min(height-2, max(10, lipgloss.Height(body)+4))
	pane := WrapInTitledBorder(body, modalW, modalH, title, true, m.theme)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, pane,
		lipgloss.WithWhitespaceBackground(m.theme.Background))
}
