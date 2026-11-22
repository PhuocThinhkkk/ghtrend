package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressTickMsg struct {
	percent float64
	message string
}

type reposLoadedMsg struct {
	repos []Repo
	err   error
}

type loadingModel struct {
	spinner  spinner.Model
	progress progress.Model
	message  string
	percent  float64
}

var (
	loadingCardBorder = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#7D56F4")).
				Padding(1, 3)

	loadingTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00A7FF")).
				MarginBottom(1)

	loadingSubTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Italic(true).
				MarginBottom(1)

	loadingStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				MarginBottom(1)

	loadingHintStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666")).
				MarginTop(1)

	errorCardBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF5555")).
			Padding(1, 3)

	errorTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			MarginBottom(1)

	errorTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginBottom(1)
)

func newLoadingModel() loadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#BA8E23")).Bold(true)

	p := progress.New(
		progress.WithGradient("#7D56F4", "#00A7FF"),
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
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return progressTickMsg{
			percent: percent,
			message: msg,
		}
	})
}

func (m Model) renderLoadingView(width, height int) string {
	title := loadingTitleStyle.Render("★  G H T R E N D  ★")
	subtitle := loadingSubTitleStyle.Render("Explore GitHub Trending directly in your terminal")

	status := loadingStatusStyle.Render(fmt.Sprintf("%s %s", m.loading.spinner.View(), m.loading.message))
	progressBar := m.loading.progress.View()
	hint := loadingHintStyle.Render("[q] Quit")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		subtitle,
		"",
		status,
		progressBar,
		hint,
	)

	card := loadingCardBorder.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

func (m Model) renderErrorView(width, height int) string {
	title := errorTitleStyle.Render("❌ Failed to load repositories")
	errMsg := "Unknown error occurred"
	if m.err != nil {
		errMsg = m.err.Error()
	}
	errText := errorTextStyle.Render(errMsg)
	hint := loadingHintStyle.Render("[q] Quit  [r] Retry")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		errText,
		"",
		hint,
	)

	card := errorCardBorder.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}
