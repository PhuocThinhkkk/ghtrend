package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type wsCommitsLoadedMsg struct {
	seq     uint64
	refSeq  uint64
	reqSeq  uint64
	ref     string
	page    int
	commits []ghclient.CommitSummary
	hasNext bool
	err     error
}

type wsCommitDiffLoadedMsg struct {
	seq    uint64
	reqSeq uint64
	sha    string
	diff   ghclient.CommitDiff
	err    error
}

type wsStarStatusLoadedMsg struct {
	seq     uint64
	starSeq uint64
	starred bool
	err     error
}

type wsStarToggledMsg struct {
	seq     uint64
	starSeq uint64
	starred bool
	err     error
}

type wsCloneCompletedMsg struct {
	seq        uint64
	targetPath string
	err        error
}

func wsFetchCommitsCmd(svc WorkspaceService, seq, refSeq, reqSeq uint64, owner, repo, ref string, page int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		commits, hasNext, err := svc.FetchCommits(ctx, owner, repo, ref, page)
		return wsCommitsLoadedMsg{seq: seq, refSeq: refSeq, reqSeq: reqSeq, ref: ref, page: page, commits: commits, hasNext: hasNext, err: err}
	}
}

func wsFetchCommitDiffCmd(svc WorkspaceService, seq, reqSeq uint64, owner, repo, sha string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		diff, err := svc.FetchCommitDiff(ctx, owner, repo, sha)
		return wsCommitDiffLoadedMsg{seq: seq, reqSeq: reqSeq, sha: sha, diff: diff, err: err}
	}
}

func wsFetchStarStatusCmd(svc WorkspaceService, seq, starSeq uint64, owner, repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		starred, err := svc.IsStarred(ctx, owner, repo)
		return wsStarStatusLoadedMsg{seq: seq, starSeq: starSeq, starred: starred, err: err}
	}
}

func wsToggleStarCmd(svc WorkspaceService, seq, starSeq uint64, owner, repo string, target bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := svc.SetStarred(ctx, owner, repo, target)
		return wsStarToggledMsg{seq: seq, starSeq: starSeq, starred: target, err: err}
	}
}

func wsCloneRepoCmd(svc WorkspaceService, seq uint64, owner, repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		target, err := svc.CloneRepo(ctx, owner, repo, ".")
		return wsCloneCompletedMsg{seq: seq, targetPath: target, err: err}
	}
}

func (m *Model) ensureWorkspaceCommitsLoaded() tea.Cmd {
	if m.workspace.commitsLoaded || m.workspace.commitsLoading {
		return nil
	}
	if m.workspace.commitPage < 1 {
		m.workspace.commitPage = 1
	}
	m.workspace.commitSeq++
	m.workspace.commitsLoading = true
	m.workspace.commitsErr = nil
	ref := m.workspace.activeRef
	return wsFetchCommitsCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.refSeq, m.workspace.commitSeq, m.workspace.owner, m.workspace.repo, ref, m.workspace.commitPage)
}

func (m *Model) reloadWorkspaceCommits(page int) tea.Cmd {
	if page < 1 {
		page = 1
	}
	m.workspace.commitSeq++
	m.workspace.commitPage = page
	m.workspace.commitsLoading = true
	m.workspace.commitsErr = nil
	m.workspace.commitCursor = 0
	m.workspace.commitOffset = 0
	return wsFetchCommitsCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.refSeq, m.workspace.commitSeq, m.workspace.owner, m.workspace.repo, m.workspace.activeRef, page)
}

func (m *Model) openCommitDiff(c ghclient.CommitSummary) tea.Cmd {
	m.workspace.commitDiffSeq++
	m.workspace.commitDiffOpen = true
	m.workspace.commitDiffLoading = true
	m.workspace.commitDiffErr = nil
	m.workspace.commitDiffFocus = wsFocusTree
	m.workspace.commitFileCursor = 0
	m.workspace.commitFileOffset = 0
	m.workspace.commitDiff = ghclient.CommitDiff{
		SHA:         c.SHA,
		ShortSHA:    c.ShortSHA,
		Message:     c.Message,
		Author:      c.Author,
		AuthorLogin: c.AuthorLogin,
		Date:        c.Date,
		HTMLURL:     c.HTMLURL,
	}
	m.workspace.commitDiffViewport.SetContent("_Loading commit diff..._")
	m.workspace.commitDiffViewport.GotoTop()
	return wsFetchCommitDiffCmd(m.activeWorkspaceService(), m.workspace.seq, m.workspace.commitDiffSeq, m.workspace.owner, m.workspace.repo, c.SHA)
}

func (m *Model) refreshCommitDiffViewport() {
	files := m.workspace.commitDiff.Files
	if len(files) == 0 {
		if m.workspace.commitDiffLoading {
			m.workspace.commitDiffViewport.SetContent("_Loading commit diff..._")
		} else if m.workspace.commitDiffErr != nil {
			m.workspace.commitDiffViewport.SetContent("⚠ " + m.workspace.commitDiffErr.Error())
		} else {
			m.workspace.commitDiffViewport.SetContent("_No changed files in this commit._")
		}
		return
	}
	idx := m.workspace.commitFileCursor
	if idx < 0 || idx >= len(files) {
		idx = 0
	}
	f := files[idx]
	w := max(24, m.workspace.commitDiffViewport.Width)
	header := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Primary).Render(
		fmt.Sprintf("%s (%s)  +%d / -%d", f.Filename, strings.ToUpper(f.Status), f.Additions, f.Deletions),
	)
	if f.PreviousFilename != "" {
		header += lipgloss.NewStyle().Foreground(m.theme.Muted).Render("  ← " + f.PreviousFilename)
	}
	diffBody := RenderUnifiedDiff(f.Patch, m.theme, w)
	m.workspace.commitDiffViewport.SetContent(header + "\n\n" + diffBody)
	m.workspace.commitDiffViewport.GotoTop()
}

func (m *Model) updateWorkspaceCommits(msg tea.KeyMsg) tea.Cmd {
	if m.workspace.commitDiffOpen {
		switch msg.String() {
		case "esc", "backspace":
			m.workspace.commitDiffSeq++
			m.workspace.commitDiffOpen = false
			m.workspace.commitDiffLoading = false
			m.workspace.commitDiffErr = nil
			return nil
		case "left", "h":
			m.workspace.commitDiffFocus = wsFocusTree
			return nil
		case "right", "l":
			m.workspace.commitDiffFocus = wsFocusViewer
			return nil
		}

		if m.workspace.commitDiffFocus == wsFocusTree {
			visRows := m.workspaceTreeVisibleRows()
			switch msg.String() {
			case "up", "k":
				if m.workspace.commitFileCursor > 0 {
					m.workspace.commitFileCursor--
					m.workspace.clampCommitFileCursor(visRows)
					m.refreshCommitDiffViewport()
				}
				return nil
			case "down", "j":
				if m.workspace.commitFileCursor < len(m.workspace.commitDiff.Files)-1 {
					m.workspace.commitFileCursor++
					m.workspace.clampCommitFileCursor(visRows)
					m.refreshCommitDiffViewport()
				}
				return nil
			case "enter":
				m.workspace.commitDiffFocus = wsFocusViewer
				return nil
			}
			return nil
		}

		switch msg.String() {
		case "up", "k":
			m.workspace.commitDiffViewport.ScrollUp(1)
			return nil
		case "down", "j":
			m.workspace.commitDiffViewport.ScrollDown(1)
			return nil
		case "pgup", "u":
			m.workspace.commitDiffViewport.HalfPageUp()
			return nil
		case "pgdown", "d":
			m.workspace.commitDiffViewport.HalfPageDown()
			return nil
		case "g":
			m.workspace.commitDiffViewport.GotoTop()
			return nil
		case "G":
			m.workspace.commitDiffViewport.GotoBottom()
			return nil
		}
		var cmd tea.Cmd
		m.workspace.commitDiffViewport, cmd = m.workspace.commitDiffViewport.Update(msg)
		return cmd
	}

	visRows := m.workspaceListVisibleRows()
	switch msg.String() {
	case "r":
		return m.reloadWorkspaceCommits(m.workspace.commitPage)
	case "]", "n":
		if m.workspace.commitHasNext {
			return m.reloadWorkspaceCommits(m.workspace.commitPage + 1)
		}
		return nil
	case "[", "p":
		if m.workspace.commitPage > 1 {
			return m.reloadWorkspaceCommits(m.workspace.commitPage - 1)
		}
		return nil
	case "up", "k":
		if m.workspace.commitCursor > 0 {
			m.workspace.commitCursor--
			m.workspace.clampCommitCursor(visRows)
		}
		return nil
	case "down", "j":
		if m.workspace.commitCursor < len(m.workspace.commits)-1 {
			m.workspace.commitCursor++
			m.workspace.clampCommitCursor(visRows)
		}
		return nil
	case "g":
		m.workspace.commitCursor = 0
		m.workspace.commitOffset = 0
		return nil
	case "G":
		if len(m.workspace.commits) > 0 {
			m.workspace.commitCursor = len(m.workspace.commits) - 1
			m.workspace.clampCommitCursor(visRows)
		}
		return nil
	case "enter", "l":
		if m.workspace.commitCursor >= 0 && m.workspace.commitCursor < len(m.workspace.commits) {
			return m.openCommitDiff(m.workspace.commits[m.workspace.commitCursor])
		}
		return nil
	}
	return nil
}

func (w *workspaceModel) clampCommitCursor(visRows int) {
	if len(w.commits) == 0 {
		w.commitCursor = 0
		w.commitOffset = 0
		return
	}
	if w.commitCursor < 0 {
		w.commitCursor = 0
	}
	if w.commitCursor >= len(w.commits) {
		w.commitCursor = len(w.commits) - 1
	}
	if visRows < 1 {
		visRows = 1
	}
	if w.commitCursor < w.commitOffset {
		w.commitOffset = w.commitCursor
	}
	if w.commitCursor >= w.commitOffset+visRows {
		w.commitOffset = w.commitCursor - visRows + 1
	}
	maxOff := max(0, len(w.commits)-visRows)
	if w.commitOffset > maxOff {
		w.commitOffset = maxOff
	}
}

func (w *workspaceModel) clampCommitFileCursor(visRows int) {
	if len(w.commitDiff.Files) == 0 {
		w.commitFileCursor = 0
		w.commitFileOffset = 0
		return
	}
	if w.commitFileCursor < 0 {
		w.commitFileCursor = 0
	}
	if w.commitFileCursor >= len(w.commitDiff.Files) {
		w.commitFileCursor = len(w.commitDiff.Files) - 1
	}
	if visRows < 1 {
		visRows = 1
	}
	if w.commitFileCursor < w.commitFileOffset {
		w.commitFileOffset = w.commitFileCursor
	}
	if w.commitFileCursor >= w.commitFileOffset+visRows {
		w.commitFileOffset = w.commitFileCursor - visRows + 1
	}
	maxOff := max(0, len(w.commitDiff.Files)-visRows)
	if w.commitFileOffset > maxOff {
		w.commitFileOffset = maxOff
	}
}

func (m Model) renderWorkspaceCommitsTab(width, availH int) string {
	if m.workspace.commitDiffOpen {
		return m.renderWorkspaceCommitDiff(width, availH)
	}

	innerW := max(16, width-4)
	refLabel := m.workspaceActiveRefOrHEAD()
	bar := TruncateDisplayWidth(
		lipgloss.NewStyle().Foreground(m.theme.Secondary).Bold(true).Render("Branch: "+refLabel)+
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render(fmt.Sprintf("   Page %d   (Enter diff • [/] page • r refresh)", max(1, m.workspace.commitPage))),
		innerW,
		"…",
	)

	var lines []string
	lines = append(lines, bar)
	lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", innerW)))

	if m.workspace.commitsLoading {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Loading commits..."))
	} else if m.workspace.commitsErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("⚠ "+m.workspace.commitsErr.Error()))
	} else if len(m.workspace.commits) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No commits found."))
	} else {
		visRows := m.workspaceListVisibleRows()
		end := min(len(m.workspace.commits), m.workspace.commitOffset+visRows)
		for i := m.workspace.commitOffset; i < end; i++ {
			c := m.workspace.commits[i]
			selected := i == m.workspace.commitCursor
			lines = append(lines, m.renderCommitRow(c, innerW, selected))
		}
	}

	title := fmt.Sprintf("Commits (%s)", refLabel)
	return WrapInTitledBorder(strings.Join(lines, "\n"), width, availH, title, true, m.theme)
}

func (m Model) renderCommitRow(c ghclient.CommitSummary, width int, selected bool) string {
	author := c.AuthorLogin
	if author == "" {
		author = c.Author
	}
	if author == "" {
		author = "unknown"
	} else if c.AuthorLogin != "" {
		author = "@" + c.AuthorLogin
	}
	rel := formatRelativeCommitTime(c.Date)
	right := fmt.Sprintf("%s · %s", author, rel)
	rightW := lipgloss.Width(right)

	sha := c.ShortSHA
	if sha == "" && len(c.SHA) >= 7 {
		sha = c.SHA[:7]
	}
	leftMax := max(14, width-rightW-3)
	left := TruncateDisplayWidth(fmt.Sprintf("%s  %s", sha, c.Message), leftMax, "…")
	gap := max(1, width-lipgloss.Width(left)-rightW)
	row := TruncateDisplayWidth(left+strings.Repeat(" ", gap)+right, width, "…")

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

func (m Model) renderWorkspaceCommitDiff(width, availH int) string {
	d := m.workspace.commitDiff
	filesTitle := fmt.Sprintf("Files (%d) · +%d/-%d", len(d.Files), d.Additions, d.Deletions)
	subj, _, _ := strings.Cut(d.Message, "\n")
	diffTitle := fmt.Sprintf("Commit %s: %s", d.ShortSHA, TruncateDisplayWidth(strings.TrimSpace(subj), max(14, width/2-20), "…"))

	renderFilesPane := func(pw, ph int) string {
		innerW := max(12, pw-4)
		visRows := max(1, ph-2)
		var lines []string
		if m.workspace.commitDiffLoading {
			lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Loading files..."))
		} else if m.workspace.commitDiffErr != nil {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("⚠ "+m.workspace.commitDiffErr.Error()))
		} else if len(d.Files) == 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No changed files."))
		} else {
			end := min(len(d.Files), m.workspace.commitFileOffset+visRows)
			for i := m.workspace.commitFileOffset; i < end; i++ {
				f := d.Files[i]
				badge := "M"
				switch strings.ToLower(f.Status) {
				case "added":
					badge = "A"
				case "removed", "deleted":
					badge = "D"
				case "renamed":
					badge = "R"
				}
				stats := fmt.Sprintf("+%d/-%d", f.Additions, f.Deletions)
				maxName := max(8, innerW-len(stats)-5)
				row := TruncateDisplayWidth(fmt.Sprintf("%s %s", badge, f.Filename), maxName, "…")
				gap := max(1, innerW-lipgloss.Width(row)-len(stats))
				line := TruncateDisplayWidth(row+strings.Repeat(" ", gap)+stats, innerW, "…")
				if i == m.workspace.commitFileCursor {
					lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(m.theme.Background).Background(m.theme.Primary).Width(innerW).Render(line))
				} else {
					lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Text).Width(innerW).Render(line))
				}
			}
		}
		return WrapInTitledBorder(strings.Join(lines, "\n"), pw, ph, filesTitle, m.workspace.commitDiffFocus == wsFocusTree, m.theme)
	}

	if width >= 95 {
		filesW := max(26, min(44, width*32/100))
		diffW := width - filesW
		leftPane := renderFilesPane(filesW, availH)
		rightPane := WrapInTitledBorder(m.workspace.commitDiffViewport.View(), diffW, availH, diffTitle, m.workspace.commitDiffFocus == wsFocusViewer, m.theme)
		return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	filesH := max(5, availH*40/100)
	diffH := max(4, availH-filesH)
	topPane := renderFilesPane(width, filesH)
	botPane := WrapInTitledBorder(m.workspace.commitDiffViewport.View(), width, diffH, diffTitle, m.workspace.commitDiffFocus == wsFocusViewer, m.theme)
	return lipgloss.JoinVertical(lipgloss.Left, topPane, botPane)
}

func formatRelativeCommitTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func (m *Model) copyWorkspaceCloneCommand() tea.Cmd {
	cloneURL, err := utils.RepoCloneURL(m.workspace.owner, m.workspace.repo)
	if err != nil {
		return m.setTransientToast("⚠ "+err.Error(), 2*time.Second)
	}
	cmdStr := "git clone " + cloneURL
	if err := utils.CopyToClipboard(cmdStr); err != nil {
		return m.setTransientToast("❌ Failed to copy clone command", 2*time.Second)
	}
	return m.setTransientToast("📋 Copied: "+cmdStr, 2*time.Second)
}
