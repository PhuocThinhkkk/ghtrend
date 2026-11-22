package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type activeComponent int8

const (
	tableActive activeComponent = iota
	listActive
	readmeActive
)

type Repo = ghclient.Repo
type EntryInfor = ghclient.EntryInfor
type ExtraInfor = ghclient.ExtraInfor

type Model struct {
	table    table.Model
	list     list.Model
	repoList []Repo
	viewport viewport.Model
	active   activeComponent
	width    int
	height   int
}

func (m *Model) getCursorRepo() Repo {
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.repoList) {
		if len(m.repoList) > 0 {
			return m.repoList[0]
		}
		return Repo{}
	}
	return m.repoList[cursor]
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.active = (m.active + 1) % 3
			return m, nil
		case "shift+tab":
			m.active = (m.active + 2) % 3
			return m, nil
		case "1":
			m.active = tableActive
			return m, nil
		case "2":
			m.active = listActive
			return m, nil
		case "3":
			m.active = readmeActive
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.recalculateLayout(msg.Width, msg.Height)
		return m, nil
	}

	prevCursor := m.table.Cursor()

	switch m.active {
	case tableActive:
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "up" || km.String() == "down") {
			if (km.String() == "up" && m.table.Cursor() == 0) ||
				(km.String() == "down" && m.table.Cursor() == len(m.table.Rows())-1) {
				return m, cmd
			}
		}
		m.table, cmd = m.table.Update(msg)

	case listActive:
		lenList := len(m.getCursorRepo().RootInfor)
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "up" || km.String() == "down") {
			if (km.String() == "up" && m.list.Index() == 0) ||
				(km.String() == "down" && m.list.Index() == lenList-1) {
				return m, cmd
			}
		}
		m.list, cmd = m.list.Update(msg)

	case readmeActive:
		m.viewport, cmd = m.viewport.Update(msg)
	}

	if m.table.Cursor() != prevCursor {
		m.setFileList()
		m.updateReadmeContent()
	}

	return m, cmd
}

func (m *Model) recalculateLayout(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	m.width = width
	m.height = height

	isWide := width >= 110

	if isWide {
		leftWidth := max(55, min(85, width*52/100))
		rightWidth := width - leftWidth - 3

		tableHeight := max(6, min(12, height*36/100))
		lowerHeight := max(8, height-tableHeight-5)

		fileListWidth := max(22, min(36, leftWidth*40/100))

		UpdateTableDimensions(&m.table, leftWidth, tableHeight)
		m.list.SetWidth(fileListWidth)
		m.list.SetHeight(lowerHeight)

		m.viewport.Width = max(20, rightWidth-4)
		m.viewport.Height = max(6, height-5)
	} else {
		contentWidth := width - 2
		tableHeight := max(6, min(10, height*35/100))
		lowerHeight := max(8, height-tableHeight-5)

		fileListWidth := max(22, contentWidth*42/100)

		UpdateTableDimensions(&m.table, contentWidth, tableHeight)
		m.list.SetWidth(fileListWidth)
		m.list.SetHeight(lowerHeight)

		m.viewport.Width = max(20, contentWidth-4)
		m.viewport.Height = max(6, height-5)
	}

	m.setFileList()
	m.updateReadmeContent()
}

func (m *Model) updateReadmeContent() {
	if len(m.repoList) == 0 {
		return
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.repoList) {
		return
	}

	rawMD := m.repoList[cursor].ReadMe
	if strings.TrimSpace(rawMD) == "" {
		rawMD = "_No README found for this repository._"
	} else {
		rawMD = utils.CleanMarkdown(rawMD)
	}

	wrapWidth := m.viewport.Width
	if wrapWidth < 20 {
		wrapWidth = 60
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrapWidth),
		glamour.WithPreservedNewLines(),
	)
	var body string
	if err == nil {
		body, _ = r.Render(rawMD)
	} else {
		body = rawMD
	}

	m.viewport.SetContent(body)
}

func (m Model) View() string {
	if len(m.repoList) == 0 {
		return "No repositories available"
	}

	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}

	isWide := width >= 110

	var mainView string
	if isWide {
		leftWidth := max(55, min(85, width*52/100))
		rightWidth := width - leftWidth - 3

		fileListWidth := max(22, min(36, leftWidth*40/100))
		metricsWidth := leftWidth - fileListWidth - 2

		tableRendered := RenderTable(m.table, m.active, leftWidth)
		fileListRendered := RenderFileList(m.list, m.active, fileListWidth, m.list.Height())
		extra := m.renderExtraInfor(metricsWidth)
		language := m.renderLanguagesBreakDown(metricsWidth)
		metricsAndLanguages := lipgloss.JoinVertical(lipgloss.Left, extra, language)
		detailSection := lipgloss.JoinHorizontal(lipgloss.Top, fileListRendered, metricsAndLanguages)
		left := lipgloss.JoinVertical(lipgloss.Left, tableRendered, detailSection)

		readmeBorder := borderStyle
		if m.active == readmeActive {
			readmeBorder = readmeBorder.BorderForeground(lipgloss.Color("#BA8E23"))
		}
		readmeTitle := headerStyle.Render("Readme Preview:")
		readmeContent := lipgloss.JoinVertical(lipgloss.Left, readmeTitle, m.viewport.View())
		right := readmeBorder.
			Width(rightWidth).
			Height(height - 4).
			Render(readmeContent)

		mainView = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	} else {
		// Compact Mode (width < 110)
		contentWidth := width - 2
		if m.active == readmeActive {
			readmeBorder := borderStyle.BorderForeground(lipgloss.Color("#BA8E23"))
			readmeTitle := headerStyle.Render("Readme Preview (Full Screen):")
			readmeContent := lipgloss.JoinVertical(lipgloss.Left, readmeTitle, m.viewport.View())
			mainView = readmeBorder.
				Width(contentWidth).
				Height(height - 4).
				Render(readmeContent)
		} else {
			fileListWidth := max(22, contentWidth*42/100)
			metricsWidth := contentWidth - fileListWidth - 2

			tableRendered := RenderTable(m.table, m.active, contentWidth)
			fileListRendered := RenderFileList(m.list, m.active, fileListWidth, m.list.Height())
			extra := m.renderExtraInfor(metricsWidth)
			language := m.renderLanguagesBreakDown(metricsWidth)
			metricsAndLanguages := lipgloss.JoinVertical(lipgloss.Left, extra, language)
			detailSection := lipgloss.JoinHorizontal(lipgloss.Top, fileListRendered, metricsAndLanguages)
			mainView = lipgloss.JoinVertical(lipgloss.Left, tableRendered, detailSection)
		}
	}

	statusBar := m.renderStatusBar(width)
	return lipgloss.JoinVertical(lipgloss.Left, mainView, statusBar)
}

func (m Model) renderStatusBar(width int) string {
	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Padding(0, 1)

	tab1 := inactiveTabStyle.Render("1: Repos")
	tab2 := inactiveTabStyle.Render("2: Files")
	tab3 := inactiveTabStyle.Render("3: Readme")

	switch m.active {
	case tableActive:
		tab1 = activeTabStyle.Render("1: Repos")
	case listActive:
		tab2 = activeTabStyle.Render("2: Files")
	case readmeActive:
		tab3 = activeTabStyle.Render("3: Readme")
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, tab1, tab2, tab3)
	helpText := lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).Render(" [Tab] Cycle  [1-3] Focus  [↑/↓] Navigate  [q] Quit")

	gap := width - lipgloss.Width(tabs) - lipgloss.Width(helpText) - 2
	if gap < 1 {
		gap = 1
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Center, tabs, strings.Repeat(" ", gap), helpText)
	return lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color("#1E1E2E")).
		Render(bar)
}

func Render(cfg *flags.CmdConfig, repos []Repo) (tea.Model, error) {
	if len(repos) == 0 {
		fmt.Println("There was no repos match that")
		os.Exit(0)
	}

	table := InitialTable(repos, cfg.Limit)
	list := InitialFileList(repos[0].RootInfor)
	vp := viewport.New(70, 20)

	m := Model{
		table:    table,
		list:     list,
		repoList: repos,
		viewport: vp,
		active:   tableActive,
		width:    120,
		height:   30,
	}
	m.setFileList()
	m.updateReadmeContent()

	p := tea.NewProgram(m, tea.WithAltScreen())
	return p.Run()
}
