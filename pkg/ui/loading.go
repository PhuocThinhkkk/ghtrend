package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/cache"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressTickMsg struct {
	seq     uint64
	percent float64
	message string
}

type reposLoadedMsg struct {
	launchSeq           uint64
	targetID            string
	repos               []Repo
	stale               bool
	isBackgroundRefresh bool
	err                 error
	refreshFn           func() ([]Repo, error)
	historyEntry        *cache.HistoryEntry
}

type loadingModel struct {
	spinner  spinner.Model
	progress progress.Model
	message  string
	percent  float64
}

func colorOrDefault(c, fallback lipgloss.Color) lipgloss.Color {
	if strings.TrimSpace(string(c)) == "" {
		return fallback
	}
	return c
}

func hexColorOrDefault(c lipgloss.Color, fallback string) string {
	s := strings.TrimSpace(string(c))
	if strings.HasPrefix(s, "#") && (len(s) == 7 || len(s) == 4) {
		return s
	}
	return fallback
}

func newLoadingModel() loadingModel {
	return newLoadingModelWithTheme(theme.Theme{})
}

func newLoadingModelWithTheme(th theme.Theme) loadingModel {
	spinnerColor := colorOrDefault(th.Accent, lipgloss.Color("#BA8E23"))
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(spinnerColor).Bold(true)

	primaryHex := hexColorOrDefault(th.Primary, "#7D56F4")
	accentHex := hexColorOrDefault(th.Accent, "#00A7FF")
	p := progress.New(
		progress.WithGradient(primaryHex, accentHex),
		progress.WithWidth(44),
	)

	return loadingModel{
		spinner:  s,
		progress: p,
		message:  "Initializing and checking cache...",
		percent:  0.15,
	}
}

func (l loadingModel) Init() tea.Cmd {
	return tea.Batch(
		l.spinner.Tick,
		tickProgress(0.35, "Connecting to GitHub & fetching repositories...", 200*time.Millisecond),
	)
}

func tickProgress(percent float64, msg string, d time.Duration) tea.Cmd {
	return tickProgressWithSeq(0, percent, msg, d)
}

func tickProgressWithSeq(seq uint64, percent float64, msg string, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return progressTickMsg{
			seq:     seq,
			percent: percent,
			message: msg,
		}
	})
}

func loadingCardLayout(width, height int) (padY, padX, innerW, availH int) {
	padY = 1
	padX = 3
	if width > 0 {
		if width < 24 {
			padX = 1
		}
		if width < 10 {
			padX = 0
		}
		chromeW := 2 + 2*padX
		innerW = min(56, max(1, width-chromeW))
	} else {
		innerW = 52
	}

	if height > 0 {
		if height < 14 {
			padY = 0
		}
		chromeH := 2 + 2*padY
		availH = max(1, height-chromeH)
	} else {
		availH = 12
	}
	return padY, padX, innerW, availH
}

func (m Model) renderLoadingView(width, height int) string {
	padY, padX, innerW, availH := loadingCardLayout(width, height)

	borderColor := colorOrDefault(m.theme.Border, lipgloss.Color("#7D56F4"))
	titleColor := colorOrDefault(m.theme.Primary, lipgloss.Color("#00A7FF"))
	subtitleColor := colorOrDefault(m.theme.Secondary, lipgloss.Color("#888888"))
	statusColor := colorOrDefault(m.theme.Text, lipgloss.Color("#FFFFFF"))
	hintColor := colorOrDefault(m.theme.Muted, lipgloss.Color("#666666"))
	spinnerColor := colorOrDefault(m.theme.Accent, lipgloss.Color("#BA8E23"))

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleColor).
		Render(TruncateDisplayWidth("★  G H T R E N D  ★", innerW, "…"))

	subtitleRaw := "Explore GitHub Trending directly in your terminal"
	if m.cfg != nil && m.cfg.User != "" {
		subtitleRaw = fmt.Sprintf("Exploring repositories of @%s", m.cfg.User)
	} else if m.cfg != nil && m.cfg.DirectRepo != "" {
		subtitleRaw = fmt.Sprintf("Opening repository %s", m.cfg.DirectRepo)
	}
	subtitle := lipgloss.NewStyle().
		Foreground(subtitleColor).
		Italic(true).
		Render(TruncateDisplayWidth(subtitleRaw, innerW, "…"))

	sp := m.loading.spinner
	sp.Style = lipgloss.NewStyle().Foreground(spinnerColor).Bold(true)
	spinnerView := sp.View()
	msgBudget := max(1, innerW-lipgloss.Width(spinnerView)-1)
	statusMsg := TruncateDisplayWidth(m.loading.message, msgBudget, "…")
	statusLine := TruncateDisplayWidth(fmt.Sprintf("%s %s", spinnerView, statusMsg), innerW, "")
	status := lipgloss.NewStyle().
		Foreground(statusColor).
		Render(statusLine)

	prog := m.loading.progress
	if prog.Width <= 0 {
		prog = newLoadingModelWithTheme(m.theme).progress
	}
	if prog.Width > innerW {
		prog.Width = innerW
	}
	progressBar := TruncateDisplayWidth(prog.View(), innerW, "")

	hint := lipgloss.NewStyle().
		Foreground(hintColor).
		Render(TruncateDisplayWidth("[q] Quit", innerW, "…"))

	var lines []string
	switch {
	case availH >= 10:
		lines = []string{title, "", subtitle, "", "", status, "", progressBar, "", hint}
	case availH >= 7:
		lines = []string{title, subtitle, "", status, progressBar, "", hint}
	case availH >= 5:
		lines = []string{title, subtitle, status, progressBar, hint}
	default:
		compact := []string{title, status, progressBar, hint}
		lines = compact[:min(availH, len(compact))]
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	card := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(padY, padX).
		Render(content)

	if width > 0 && height > 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
	}
	return card
}

func wrapAndClampLines(text string, maxW, maxLines int) []string {
	if maxLines <= 0 || maxW <= 0 {
		return nil
	}
	wrapped := lipgloss.NewStyle().Width(maxW).Render(text)
	rawLines := strings.Split(wrapped, "\n")
	lines := make([]string, 0, min(len(rawLines), maxLines))
	for _, line := range rawLines {
		trimmed := strings.TrimRight(line, " ")
		if lipgloss.Width(trimmed) > maxW {
			trimmed = TruncateDisplayWidth(trimmed, maxW, "…")
		}
		lines = append(lines, trimmed)
		if len(lines) == maxLines {
			if len(rawLines) > maxLines {
				lines[maxLines-1] = TruncateDisplayWidth(lines[maxLines-1], max(1, maxW-1), "") + "…"
				if lipgloss.Width(lines[maxLines-1]) > maxW {
					lines[maxLines-1] = TruncateDisplayWidth(lines[maxLines-1], maxW, "")
				}
			}
			break
		}
	}
	return lines
}

func (m Model) renderErrorView(width, height int) string {
	padY, padX, innerW, availH := loadingCardLayout(width, height)

	borderColor := colorOrDefault(m.theme.Border, lipgloss.Color("#FF5555"))
	titleColor := lipgloss.Color("#FF5555")
	textColor := colorOrDefault(m.theme.Text, lipgloss.Color("#FFFFFF"))
	hintColor := colorOrDefault(m.theme.Muted, lipgloss.Color("#666666"))

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleColor).
		Render(TruncateDisplayWidth("❌ Failed to load repositories", innerW, "…"))

	errMsg := "Unknown error occurred"
	if m.err != nil {
		errMsg = m.err.Error()
	}

	hint := lipgloss.NewStyle().
		Foreground(hintColor).
		Render(TruncateDisplayWidth("[q] Quit  [r] Retry  [H] Home", innerW, "…"))

	errStyle := lipgloss.NewStyle().Foreground(textColor)

	var lines []string
	switch {
	case availH >= 5:
		maxErrLines := max(1, availH-4)
		errLines := wrapAndClampLines(errMsg, innerW, maxErrLines)
		errBlock := errStyle.Render(strings.Join(errLines, "\n"))
		lines = []string{title, "", errBlock, "", hint}
	case availH >= 3:
		maxErrLines := max(1, availH-2)
		errLines := wrapAndClampLines(errMsg, innerW, maxErrLines)
		errBlock := errStyle.Render(strings.Join(errLines, "\n"))
		lines = []string{title, errBlock, hint}
	case availH == 2:
		lines = []string{title, hint}
	default:
		lines = []string{title}
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	card := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(padY, padX).
		Render(content)

	if width > 0 && height > 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
	}
	return card
}
