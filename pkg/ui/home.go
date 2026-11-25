package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ascii"
	"github.com/PhuocThinhkkk/ghtrend/pkg/cache"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type homeFocus int8

const (
	homeFocusMenu homeFocus = iota
	homeFocusHistory
)

type homeInputMode int8

const (
	homeInputNone homeInputMode = iota
	homeInputUser
	homeInputRepo
	homeInputLang
)

type homeMenuAction int

const (
	homeActionTrending homeMenuAction = iota
	homeActionUser
	homeActionRepo
)

const homeMenuCount = 3

type homeModel struct {
	focus             homeFocus
	menuIndex         int
	historyIndex      int
	historyOffset     int
	history           []cache.HistoryEntry
	historyLoadFailed bool
	inputMode         homeInputMode
	input             textinput.Model
	inputError        string

	trendSince flags.Frequency
	trendLang  string
}

type historyLoadedMsg struct {
	seq     uint64
	entries []cache.HistoryEntry
	err     error
}

type historyActionDoneMsg struct {
	seq     uint64
	entries []cache.HistoryEntry
	toast   string
	err     error
}

func newHomeModel(cfg *flags.CmdConfig) homeModel {
	since := flags.Daily
	lang := "All"
	if cfg != nil {
		if flags.IsValidFrequency(cfg.Since) {
			since = cfg.Since
		}
		if strings.TrimSpace(cfg.Language) != "" {
			lang = strings.TrimSpace(cfg.Language)
		}
	}

	ti := textinput.New()
	ti.Prompt = "▶ "
	ti.CharLimit = 128
	ti.Width = 34

	return homeModel{
		focus:      homeFocusMenu,
		trendSince: since,
		trendLang:  lang,
		input:      ti,
		history:    []cache.HistoryEntry{},
	}
}

func (h homeModel) isInputActive() bool {
	return h.inputMode != homeInputNone
}

func (h *homeModel) beginInput(mode homeInputMode, placeholder, initial string) tea.Cmd {
	h.inputMode = mode
	h.inputError = ""
	h.input.Placeholder = placeholder
	h.input.SetValue(initial)
	h.input.CursorEnd()
	h.input.Focus()
	return textinput.Blink
}

func (h *homeModel) cancelInput() {
	h.inputMode = homeInputNone
	h.inputError = ""
	h.input.Reset()
	h.input.Blur()
}

func (h *homeModel) clampHistorySelection(visibleRows int) {
	if len(h.history) == 0 {
		h.historyIndex = 0
		h.historyOffset = 0
		return
	}
	if h.historyIndex < 0 {
		h.historyIndex = 0
	}
	if h.historyIndex >= len(h.history) {
		h.historyIndex = len(h.history) - 1
	}
	if visibleRows < 1 {
		visibleRows = 1
	}
	if h.historyIndex < h.historyOffset {
		h.historyOffset = h.historyIndex
	}
	if h.historyIndex >= h.historyOffset+visibleRows {
		h.historyOffset = h.historyIndex - visibleRows + 1
	}
	maxOffset := max(0, len(h.history)-visibleRows)
	if h.historyOffset > maxOffset {
		h.historyOffset = maxOffset
	}
}

func loadHistoryCmd(store *cache.HistoryStore, seq uint64) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return historyLoadedMsg{seq: seq, entries: []cache.HistoryEntry{}}
		}
		entries, err := store.Load()
		return historyLoadedMsg{seq: seq, entries: entries, err: err}
	}
}

func recordHistoryCmd(store *cache.HistoryStore, entry cache.HistoryEntry, seq uint64) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return nil
		}
		if err := store.Record(entry); err != nil {
			return historyActionDoneMsg{seq: seq, err: err}
		}
		entries, err := store.Load()
		if err != nil {
			return historyActionDoneMsg{seq: seq, err: err}
		}
		return historyLoadedMsg{seq: seq, entries: entries}
	}
}

func deleteHistoryCmd(store *cache.HistoryStore, id string, seq uint64) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return historyActionDoneMsg{seq: seq, entries: []cache.HistoryEntry{}}
		}
		if err := store.Delete(id); err != nil {
			return historyActionDoneMsg{seq: seq, err: err}
		}
		entries, err := store.Load()
		return historyActionDoneMsg{
			seq:     seq,
			entries: entries,
			toast:   "🗑 Removed from history",
			err:     err,
		}
	}
}

func clearHistoryCmd(store *cache.HistoryStore, seq uint64) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return historyActionDoneMsg{seq: seq, entries: []cache.HistoryEntry{}}
		}
		if err := store.Clear(); err != nil {
			return historyActionDoneMsg{seq: seq, err: err}
		}
		return historyActionDoneMsg{
			seq:     seq,
			entries: []cache.HistoryEntry{},
			toast:   "🧹 History cleared",
		}
	}
}

func (m *Model) updateHome(msg tea.KeyMsg) tea.Cmd {
	if m.home.isInputActive() {
		switch msg.String() {
		case "esc":
			m.home.cancelInput()
			return nil
		case "enter":
			return m.submitHomeInput()
		default:
			var cmd tea.Cmd
			m.home.input, cmd = m.home.input.Update(msg)
			m.home.inputError = ""
			return cmd
		}
	}

	switch msg.String() {
	case "tab", "shift+tab":
		if m.home.focus == homeFocusMenu {
			m.home.focus = homeFocusHistory
		} else {
			m.home.focus = homeFocusMenu
		}
		return nil
	case "1":
		m.home.focus = homeFocusMenu
		return nil
	case "2":
		m.home.focus = homeFocusHistory
		return nil
	case "u":
		m.home.focus = homeFocusMenu
		m.home.menuIndex = int(homeActionUser)
		return m.home.beginInput(homeInputUser, "@username (e.g. torvalds)", "")
	case "g":
		m.home.focus = homeFocusMenu
		m.home.menuIndex = int(homeActionRepo)
		return m.home.beginInput(homeInputRepo, "owner/repo or GitHub URL", "")
	}

	if m.home.focus == homeFocusMenu {
		switch msg.String() {
		case "up", "k":
			m.home.menuIndex = (m.home.menuIndex - 1 + homeMenuCount) % homeMenuCount
			return nil
		case "down", "j":
			m.home.menuIndex = (m.home.menuIndex + 1) % homeMenuCount
			return nil
		case "right", " ":
			if homeMenuAction(m.home.menuIndex) == homeActionTrending {
				m.home.trendSince = flags.NextFrequency(m.home.trendSince)
			}
			return nil
		case "left":
			if homeMenuAction(m.home.menuIndex) == homeActionTrending {
				m.home.trendSince = prevFrequency(m.home.trendSince)
			}
			return nil
		case "l":
			if homeMenuAction(m.home.menuIndex) == homeActionTrending {
				cur := m.home.trendLang
				if strings.EqualFold(cur, "All") {
					cur = ""
				}
				return m.home.beginInput(homeInputLang, "Language (empty for All)", cur)
			}
			return nil
		case "enter":
			return m.activateHomeMenuSelection()
		}
		return nil
	}

	// History pane focused
	switch msg.String() {
	case "up", "k":
		if m.home.historyIndex > 0 {
			m.home.historyIndex--
			m.home.clampHistorySelection(m.homeHistoryVisibleRows())
		}
		return nil
	case "down", "j":
		if m.home.historyIndex < len(m.home.history)-1 {
			m.home.historyIndex++
			m.home.clampHistorySelection(m.homeHistoryVisibleRows())
		}
		return nil
	case "enter":
		if len(m.home.history) == 0 || m.home.historyIndex < 0 || m.home.historyIndex >= len(m.home.history) {
			return nil
		}
		entry := m.home.history[m.home.historyIndex]
		return m.openHistoryEntry(entry)
	case "d", "x":
		if len(m.home.history) == 0 || m.home.historyIndex < 0 || m.home.historyIndex >= len(m.home.history) {
			return nil
		}
		entry := m.home.history[m.home.historyIndex]
		m.historyReqSeq++
		return deleteHistoryCmd(m.launchOpts.HistoryStore, entry.ID, m.historyReqSeq)
	case "D", "ctrl+x":
		if len(m.home.history) == 0 && !m.home.historyLoadFailed {
			return nil
		}
		m.historyReqSeq++
		return clearHistoryCmd(m.launchOpts.HistoryStore, m.historyReqSeq)
	}

	return nil
}

func prevFrequency(f flags.Frequency) flags.Frequency {
	switch f {
	case flags.Daily:
		return flags.Monthly
	case flags.Weekly:
		return flags.Daily
	case flags.Monthly:
		return flags.Weekly
	default:
		return flags.Daily
	}
}

func (m *Model) activateHomeMenuSelection() tea.Cmd {
	switch homeMenuAction(m.home.menuIndex) {
	case homeActionTrending:
		return m.launchTrending(m.home.trendSince, m.home.trendLang)
	case homeActionUser:
		return m.home.beginInput(homeInputUser, "@username (e.g. torvalds)", "")
	case homeActionRepo:
		return m.home.beginInput(homeInputRepo, "owner/repo or GitHub URL", "")
	default:
		return nil
	}
}

func (m *Model) submitHomeInput() tea.Cmd {
	raw := strings.TrimSpace(m.home.input.Value())
	mode := m.home.inputMode

	switch mode {
	case homeInputLang:
		if raw == "" {
			m.home.trendLang = "All"
		} else {
			m.home.trendLang = raw
		}
		m.home.cancelInput()
		return nil

	case homeInputUser:
		username, ok := cache.ParseUserTarget(raw)
		if !ok {
			m.home.inputError = "Enter a valid GitHub username (e.g. torvalds)"
			return nil
		}
		m.home.cancelInput()
		return m.launchUser(username, "All")

	case homeInputRepo:
		owner, repo, ok := cache.ParseRepoSlug(raw)
		if !ok {
			m.home.inputError = "Enter owner/repo or a github.com URL"
			return nil
		}
		m.home.cancelInput()
		return m.launchDirectRepo(owner, repo)
	}

	m.home.cancelInput()
	return nil
}

func (m *Model) openHistoryEntry(entry cache.HistoryEntry) tea.Cmd {
	switch entry.Type {
	case cache.HistoryTypeTrend:
		since := entry.Frequency
		if !flags.IsValidFrequency(since) {
			since = flags.Daily
		}
		lang := strings.TrimSpace(entry.Language)
		if lang == "" {
			lang = "All"
		}
		m.home.trendSince = since
		m.home.trendLang = lang
		return m.launchTrending(since, lang)

	case cache.HistoryTypeUser:
		username, ok := cache.ParseUserTarget(entry.Target)
		if !ok {
			return m.setTransientToast("❌ Invalid user in history", 2*time.Second)
		}
		lang := strings.TrimSpace(entry.Language)
		if lang == "" {
			lang = "All"
		}
		return m.launchUser(username, lang)

	case cache.HistoryTypeRepo:
		owner, repo, ok := cache.ParseRepoSlug(entry.Target)
		if !ok {
			return m.setTransientToast("❌ Invalid repo in history", 2*time.Second)
		}
		return m.launchDirectRepo(owner, repo)
	}
	return nil
}

func (m Model) homeHistoryVisibleRows() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	availH := h - 1
	if m.width >= 100 {
		bannerH := 0
		if h >= 24 {
			bannerH = 4
		}
		return max(3, availH-bannerH-4)
	}
	hubH := min(11, max(7, availH*48/100))
	histH := availH - hubH
	return max(2, histH-4)
}

func (m Model) renderHomeView(width, height int) string {
	availH := max(6, height-1)
	isWide := width >= 100

	var sections []string
	contentH := availH

	if isWide && height >= 24 {
		banner := m.renderHomeBanner(width)
		bannerH := lipgloss.Height(banner)
		sections = append(sections, banner)
		contentH = max(8, availH-bannerH)
	}

	var body string
	if isWide {
		leftW := max(42, min(58, width*44/100))
		rightW := width - leftW
		hubBox := m.renderHomeMenuBox(leftW, contentH)
		histBox := m.renderHomeHistoryBox(rightW, contentH)
		body = lipgloss.JoinHorizontal(lipgloss.Top, hubBox, histBox)
	} else {
		hubH := min(12, max(7, contentH*50/100))
		if m.home.isInputActive() && contentH >= 13 {
			hubH = max(hubH, 8)
		}
		histH := max(5, contentH-hubH)
		hubH = contentH - histH
		hubBox := m.renderHomeMenuBox(width, hubH)
		histBox := m.renderHomeHistoryBox(width, histH)
		body = lipgloss.JoinVertical(lipgloss.Left, hubBox, histBox)
	}

	sections = append(sections, body)
	sections = append(sections, m.renderHomeStatusBar(width))
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHomeBanner(width int) string {
	bannerLines := ascii.RenderPixelBanner(
		"GHTREND",
		hexColorOrDefault(m.theme.Primary, "#7aa2f7"),
		hexColorOrDefault(m.theme.Secondary, "#bb9af7"),
		hexColorOrDefault(m.theme.Border, "#414868"),
	)
	banner := strings.Join(bannerLines[:], "\n")
	if lipgloss.Width(banner) > width-4 {
		banner = lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Primary).
			Render("★ G H T R E N D ★")
	}
	sub := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("GitHub Trending · User Profiles · Repository Explorer")

	inner := lipgloss.JoinVertical(lipgloss.Center, banner, sub)
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(inner)
}

func (m Model) renderHomeMenuBox(width, height int) string {
	focused := m.home.focus == homeFocusMenu
	innerW := max(10, width-4)

	selStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Background).
		Background(m.theme.Primary)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	mutedStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
	accentStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)

	type menuRow struct {
		icon   string
		title  string
		detail string
	}

	langDisplay := m.home.trendLang
	if strings.TrimSpace(langDisplay) == "" {
		langDisplay = "All"
	}

	items := []menuRow{
		{
			icon:   "🔥",
			title:  "GitHub Trending",
			detail: fmt.Sprintf("◀ %s ▶  ·  Lang: %s [l]", m.home.trendSince.DisplayTitle(), langDisplay),
		},
		{
			icon:   "👤",
			title:  "User Profile Explorer",
			detail: "Browse profile, heatmap & pinned repos [u]",
		},
		{
			icon:   "📦",
			title:  "Open Repository",
			detail: "Jump to any owner/repo or GitHub URL [g]",
		},
	}

	var lines []string
	compactInput := m.home.isInputActive() && height < 12
	for i, item := range items {
		isSelected := i == m.home.menuIndex
		prefix := "  "
		if isSelected {
			prefix = "▶ "
		}
		label := TruncateDisplayWidth(prefix+item.icon+" "+item.title, innerW, "…")
		if isSelected && focused {
			lines = append(lines, selStyle.Render(padRightDisplay(label, innerW)))
		} else if isSelected {
			lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render(label))
		} else {
			lines = append(lines, normalStyle.Render(label))
		}

		if !compactInput && (height >= 11 || isSelected) {
			detail := TruncateDisplayWidth("   "+item.detail, innerW, "…")
			if isSelected {
				lines = append(lines, accentStyle.Render(detail))
			} else {
				lines = append(lines, mutedStyle.Render(detail))
			}
		}
		if !compactInput && height >= 14 && i < len(items)-1 {
			lines = append(lines, "")
		}
	}

	if m.home.isInputActive() {
		label := "Input:"
		switch m.home.inputMode {
		case homeInputUser:
			label = "Username:"
		case homeInputRepo:
			label = "Repository:"
		case homeInputLang:
			label = "Language:"
		}
		if !compactInput {
			lines = append(lines, "")
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Secondary).Bold(true).Render(label))
		inputCopy := m.home.input
		inputCopy.Width = max(10, innerW-4)
		lines = append(lines, " "+inputCopy.View())
		if m.home.inputError != "" {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true)
			lines = append(lines, errStyle.Render(TruncateDisplayWidth(" ! "+m.home.inputError, innerW, "…")))
		} else if !compactInput || height >= 10 {
			lines = append(lines, mutedStyle.Render(TruncateDisplayWidth(" [Enter] Open  [Esc] Cancel", innerW, "…")))
		}
	} else if height >= 13 {
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render(TruncateDisplayWidth(" [←/→] Range  [l] Lang  [Enter] Launch", innerW, "…")))
	}

	padded := make([]string, 0, len(lines))
	for _, ln := range lines {
		padded = append(padded, " "+ln)
	}
	return WrapInTitledBorder(strings.Join(padded, "\n"), width, height, "1: Launcher Hub", focused, m.theme)
}

func (m Model) renderHomeHistoryBox(width, height int) string {
	focused := m.home.focus == homeFocusHistory
	innerW := max(10, width-4)
	visibleRows := max(1, height-4)

	mutedStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
	title := fmt.Sprintf("2: Recent History (%d)", len(m.home.history))

	if len(m.home.history) == 0 {
		emptyLines := []string{
			"",
			mutedStyle.Render(TruncateDisplayWidth(" No recent visits yet.", innerW, "…")),
			mutedStyle.Render(TruncateDisplayWidth(" Launch Trending, @user, or owner/repo to save.", innerW, "…")),
		}
		return WrapInTitledBorder(strings.Join(emptyLines, "\n"), width, height, title, focused, m.theme)
	}

	hCopy := m.home
	hCopy.clampHistorySelection(visibleRows)

	endIdx := min(len(hCopy.history), hCopy.historyOffset+visibleRows)
	var rows []string
	for i := hCopy.historyOffset; i < endIdx; i++ {
		entry := hCopy.history[i]
		selected := i == hCopy.historyIndex
		rows = append(rows, m.renderHistoryRow(entry, innerW, selected, focused))
	}

	// Footer hint if room
	if height-2 > len(rows)+1 {
		for len(rows) < height-4 {
			rows = append(rows, "")
		}
		hint := TruncateDisplayWidth(" [Enter] Reopen  [d/x] Delete  [D] Clear All", innerW, "…")
		rows = append(rows, " "+mutedStyle.Render(hint))
	}

	return WrapInTitledBorder(strings.Join(rows, "\n"), width, height, title, focused, m.theme)
}

func (m Model) renderHistoryRow(entry cache.HistoryEntry, innerW int, selected, focused bool) string {
	cursor := "  "
	if selected {
		cursor = "▶ "
	}

	badgeColor := m.theme.Primary
	switch entry.Type {
	case cache.HistoryTypeUser:
		badgeColor = m.theme.Accent
	case cache.HistoryTypeTrend:
		badgeColor = m.theme.Secondary
	}
	badgeText := fmt.Sprintf("%-5s", string(entry.Type))
	badge := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Background).
		Background(badgeColor).
		Padding(0, 1).
		Render(badgeText)

	rightMeta := formatRelativeTime(entry.LastVisited)
	if entry.VisitCount > 1 {
		rightMeta = fmt.Sprintf("×%d · %s", entry.VisitCount, rightMeta)
	}

	label := entry.Label
	if strings.TrimSpace(label) == "" {
		label = entry.Target
	}

	cursorW := lipgloss.Width(cursor)
	badgeW := lipgloss.Width(badge)
	rightW := lipgloss.Width(rightMeta)
	availLabelW := innerW - cursorW - badgeW - rightW - 2
	if availLabelW < 6 {
		rightMeta = ""
		rightW = 0
		availLabelW = max(4, innerW-cursorW-badgeW-1)
	}

	labelClipped := TruncateDisplayWidth(label, availLabelW, "…")
	labelStyled := lipgloss.NewStyle().Foreground(m.theme.Text).Render(labelClipped)
	if selected && focused {
		labelStyled = lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render(labelClipped)
	}

	left := cursor + badge + " " + labelStyled
	if rightMeta == "" {
		return " " + left
	}
	gap := max(1, innerW-lipgloss.Width(left)-rightW)
	rightStyled := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(rightMeta)
	return " " + left + strings.Repeat(" ", gap) + rightStyled
}

func (m Model) renderHomeStatusBar(width int) string {
	homePill := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Background).
		Background(m.theme.Primary).
		Padding(0, 1).
		Render("🏠 Home")

	themePill := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.Surface).
		Padding(0, 1).
		Render(fmt.Sprintf("🎨 %s", m.theme.ID))

	var toastPill string
	if m.toastMessage != "" {
		toastPill = lipgloss.NewStyle().
			Foreground(m.theme.Background).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1).
			Render(m.toastMessage)
	}

	var rateLimitPill string
	rlInfo := ghclient.GetRateLimitInfo()
	if text, isWarn, isCrit := rlInfo.FormatRateLimitIndicator(); text != "" {
		rlStyle := lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Background(m.theme.Surface).
			Padding(0, 1)
		if isCrit {
			rlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(lipgloss.Color("#f7768e")).Bold(true).Padding(0, 1)
		} else if isWarn {
			rlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(lipgloss.Color("#e0af68")).Bold(true).Padding(0, 1)
		}
		rateLimitPill = rlStyle.Render(text)
	}

	items := []string{homePill}
	if toastPill != "" {
		items = append(items, " ", toastPill)
	}
	items = append(items, " ", themePill)
	if rateLimitPill != "" {
		items = append(items, " ", rateLimitPill)
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top, items...)

	keyStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
	bullet := lipgloss.NewStyle().Foreground(m.theme.Border).Render(" • ")
	kb := func(k, d string) string {
		return keyStyle.Render(k) + " " + descStyle.Render(d)
	}

	var helpItems []string
	if width >= 115 {
		helpItems = []string{kb("Tab", "pane"), kb("Enter", "open"), kb("d", "del"), kb("D", "clear"), kb("t", "theme"), kb("?", "help"), kb("q", "quit")}
	} else if width >= 80 {
		helpItems = []string{kb("Tab", "pane"), kb("Enter", "open"), kb("t", "theme"), kb("q", "quit")}
	} else {
		helpItems = []string{kb("q", "quit")}
	}
	helpText := strings.Join(helpItems, bullet)

	gap := width - lipgloss.Width(left) - lipgloss.Width(helpText)
	if gap < 1 && width >= 80 {
		helpItems = []string{kb("Tab", "pane"), kb("Enter", "open"), kb("?", "help"), kb("q", "quit")}
		helpText = strings.Join(helpItems, bullet)
		gap = width - lipgloss.Width(left) - lipgloss.Width(helpText)
	}
	if gap < 1 {
		leftItems := []string{homePill}
		if toastPill != "" {
			leftItems = append(leftItems, " ", toastPill)
		}
		left = lipgloss.JoinHorizontal(lipgloss.Top, leftItems...)
		gap = width - lipgloss.Width(left) - lipgloss.Width(helpText)
	}
	if gap < 1 && toastPill != "" {
		helpItems = []string{kb("q", "quit")}
		helpText = strings.Join(helpItems, bullet)
		gap = width - lipgloss.Width(left) - lipgloss.Width(helpText)
	}
	if gap < 1 && toastPill != "" {
		maxToastText := width - lipgloss.Width(homePill) - lipgloss.Width(helpText) - 5
		if maxToastText >= 4 {
			clipped := TruncateDisplayWidth(m.toastMessage, maxToastText, "…")
			toastPill = lipgloss.NewStyle().
				Foreground(m.theme.Background).
				Background(m.theme.Accent).
				Bold(true).
				Padding(0, 1).
				Render(clipped)
			left = lipgloss.JoinHorizontal(lipgloss.Top, homePill, " ", toastPill)
			gap = width - lipgloss.Width(left) - lipgloss.Width(helpText)
		} else {
			left = homePill
			gap = width - lipgloss.Width(left) - lipgloss.Width(helpText)
		}
	}
	if gap < 1 {
		left = homePill
		gap = max(1, width-lipgloss.Width(left)-lipgloss.Width(helpText))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", gap), helpText)
	return lipgloss.NewStyle().
		Width(width).
		Height(1).
		MaxHeight(1).
		Background(m.theme.Background).
		Render(bar)
}

func padRightDisplay(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}
