package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/appconfig"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type appState int

const (
	stateLoading appState = iota
	stateDashboard
	stateError
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
	state          appState
	loading        loadingModel
	fetcher        func() ([]Repo, error)
	cfg            *flags.CmdConfig
	theme          theme.Theme
	showThemeModal bool
	themeIndex     int
	table          table.Model
	list           list.Model
	repoList       []Repo
	viewport       viewport.Model
	active         activeComponent
	width          int
	height         int
	err            error
	userProfile    *ghclient.UserProfile
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
	if m.state == stateLoading {
		return tea.Batch(
			m.loading.Init(),
			fetchReposCmd(m.fetcher),
		)
	}
	return nil
}

func fetchReposCmd(fetcher func() ([]Repo, error)) tea.Cmd {
	return func() tea.Msg {
		if fetcher == nil {
			return reposLoadedMsg{repos: nil, err: fmt.Errorf("no fetcher provided")}
		}
		repos, err := fetcher()
		return reposLoadedMsg{repos: repos, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showThemeModal {
			switch msg.String() {
			case "esc", "t", "q":
				m.showThemeModal = false
				return m, nil
			case "up", "k":
				m.themeIndex = (m.themeIndex - 1 + len(theme.Catalog)) % len(theme.Catalog)
				return m, nil
			case "down", "j":
				m.themeIndex = (m.themeIndex + 1) % len(theme.Catalog)
				return m, nil
			case "enter":
				selected := theme.Catalog[m.themeIndex]
				m.theme = selected
				m.showThemeModal = false
				ApplyThemeToTable(&m.table, m.theme)
				m.updateReadmeContent()
				_ = appconfig.SaveConfig(appconfig.Config{Theme: selected.ID})
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "t":
			if m.state == stateDashboard {
				m.showThemeModal = true
				for i, t := range theme.Catalog {
					if t.ID == m.theme.ID {
						m.themeIndex = i
						break
					}
				}
				return m, nil
			}
		case "r":
			if m.state == stateError {
				m.state = stateLoading
				m.loading = newLoadingModel()
				return m, tea.Batch(m.loading.Init(), fetchReposCmd(m.fetcher))
			}
		case "tab":
			if m.state == stateDashboard {
				m.active = (m.active + 1) % 3
				return m, nil
			}
		case "shift+tab":
			if m.state == stateDashboard {
				m.active = (m.active + 2) % 3
				return m, nil
			}
		case "1":
			if m.state == stateDashboard {
				m.active = tableActive
				return m, nil
			}
		case "2":
			if m.state == stateDashboard {
				m.active = listActive
				return m, nil
			}
		case "3":
			if m.state == stateDashboard {
				m.active = readmeActive
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateDashboard {
			m.recalculateLayout(msg.Width, msg.Height)
		}
		return m, nil

	case progressTickMsg:
		if m.state == stateLoading {
			m.loading.percent = msg.percent
			m.loading.message = msg.message
			cmds = append(cmds, m.loading.progress.SetPercent(msg.percent))
			if msg.percent < 0.80 {
				cmds = append(cmds, tickProgress(msg.percent+0.25, "Parsing metrics & generating views...", 300*time.Millisecond))
			}
			return m, tea.Batch(cmds...)
		}

	case reposLoadedMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
			return m, nil
		}
		if len(msg.repos) == 0 {
			m.state = stateError
			m.err = fmt.Errorf("no trending repositories found for the given criteria")
			return m, nil
		}

		limit := 2202
		if m.cfg != nil {
			limit = m.cfg.Limit
		}

		if len(msg.repos) > 0 && msg.repos[0].UserProfile != nil {
			m.userProfile = msg.repos[0].UserProfile
		}

		m.repoList = msg.repos
		m.table = InitialTable(msg.repos, limit, m.theme)
		m.list = InitialFileList(msg.repos[0].RootInfor)
		m.viewport = viewport.New(70, 20)
		m.active = tableActive
		m.recalculateLayout(m.width, m.height)
		m.state = stateDashboard
		return m, nil

	case spinner.TickMsg:
		if m.state == stateLoading {
			var spinnerCmd tea.Cmd
			m.loading.spinner, spinnerCmd = m.loading.spinner.Update(msg)
			return m, spinnerCmd
		}

	case progress.FrameMsg:
		if m.state == stateLoading {
			progressModel, progressCmd := m.loading.progress.Update(msg)
			m.loading.progress = progressModel.(progress.Model)
			return m, progressCmd
		}
	}

	if m.state != stateDashboard {
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
	availH := height - 1

	if isWide {
		leftWidth := max(55, min(85, width*52/100))
		rightWidth := width - leftWidth

		if m.userProfile != nil {
			profileH := max(8, min(13, availH*42/100))
			readmeH := availH - profileH

			tableHeight := max(6, min(12, availH*38/100))
			lowerHeight := availH - tableHeight

			fileListWidth := max(22, min(36, rightWidth*40/100))

			UpdateTableDimensions(&m.table, rightWidth, tableHeight)
			m.list.SetWidth(fileListWidth - 2)
			m.list.SetHeight(max(1, lowerHeight-2))

			m.viewport.Width = max(20, leftWidth-4)
			m.viewport.Height = max(1, readmeH-4)
		} else {
			tableHeight := max(6, min(12, availH*38/100))
			lowerHeight := availH - tableHeight

			fileListWidth := max(22, min(36, leftWidth*40/100))

			UpdateTableDimensions(&m.table, leftWidth, tableHeight)
			m.list.SetWidth(fileListWidth - 2)
			m.list.SetHeight(max(1, lowerHeight-2))

			m.viewport.Width = max(20, rightWidth-4)
			m.viewport.Height = max(1, availH-4)
		}
	} else {
		contentWidth := width
		tableHeight := max(6, min(10, availH*38/100))
		lowerHeight := availH - tableHeight

		fileListWidth := max(22, contentWidth*42/100)

		UpdateTableDimensions(&m.table, contentWidth, tableHeight)
		m.list.SetWidth(fileListWidth - 2)
		m.list.SetHeight(max(1, lowerHeight-2))

		m.viewport.Width = max(20, contentWidth-4)
		m.viewport.Height = max(1, availH-4)
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

	glamourStyle := m.theme.Glamour
	if glamourStyle == "" {
		glamourStyle = "dark"
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamourStyle),
		glamour.WithWordWrap(wrapWidth),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		r, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(wrapWidth),
			glamour.WithPreservedNewLines(),
		)
	}

	var body string
	if r != nil {
		body, _ = r.Render(rawMD)
	} else {
		body = rawMD
	}

	m.viewport.SetContent(body)
	m.viewport.GotoTop()
}

func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}

	if width < 60 || height < 14 {
		return m.renderTooSmallView(width, height)
	}

	if m.showThemeModal {
		return m.renderThemeSelectorModal(width, height)
	}

	switch m.state {
	case stateLoading:
		return m.renderLoadingView(width, height)
	case stateError:
		return m.renderErrorView(width, height)
	case stateDashboard:
		return m.renderDashboardView(width, height)
	default:
		return ""
	}
}

func (m Model) renderTooSmallView(width, height int) string {
	if height < 6 || width < 25 {
		msg := "Window too small"
		if width < 16 {
			msg = "Too small"
		}
		if height > 1 && width >= 10 {
			line2 := "Min: 60x14"
			if len(line2) > width {
				line2 = line2[:width]
			}
			msg += "\n" + line2
		}
		styled := lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(msg)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, styled)
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary).
		Render("⚠️  Window Too Small")

	sizeInfo := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render(fmt.Sprintf("Current: %d×%d", width, height))

	reqInfo := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Render("Min required: 60×14")

	action := lipgloss.NewStyle().
		Foreground(m.theme.Accent).
		Render("Please enlarge window")

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		sizeInfo,
		reqInfo,
		action,
	)

	card := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(0, 1).
		MaxHeight(height).
		MaxWidth(width).
		Align(lipgloss.Center).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

func (m Model) renderDashboardView(width, height int) string {
	if len(m.repoList) == 0 {
		return "No repositories available"
	}

	isWide := width >= 110
	availH := height - 1

	var mainView string
	if isWide {
		leftWidth := max(55, min(85, width*52/100))
		rightWidth := width - leftWidth

		if m.userProfile != nil {
			profileH := max(8, min(13, availH*42/100))
			readmeH := availH - profileH

			profileCard := m.renderUserProfileCard(leftWidth, profileH, m.theme)

			readmeBorder := lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(m.theme.Border).
				Width(leftWidth - 2).
				Height(readmeH - 2).
				MaxHeight(readmeH)

			if m.active == readmeActive {
				readmeBorder = readmeBorder.BorderForeground(m.theme.FocusBorder)
			}
			readmeTitle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render("Readme Preview:")
			readmeContent := lipgloss.JoinVertical(lipgloss.Left, readmeTitle, m.viewport.View())
			readmeRendered := readmeBorder.Render(readmeContent)

			left := lipgloss.JoinVertical(lipgloss.Left, profileCard, readmeRendered)

			tableHeight := max(6, min(12, availH*38/100))
			lowerHeight := availH - tableHeight

			fileListWidth := max(22, min(36, rightWidth*40/100))
			metricsWidth := rightWidth - fileListWidth

			tableRendered := RenderTable(m.table, m.active, rightWidth, tableHeight, m.theme)
			fileListRendered := RenderFileList(m.list, m.active, fileListWidth, lowerHeight, m.theme)

			extraH := 7
			if lowerHeight < 14 {
				extraH = max(4, lowerHeight/2)
			}
			langH := lowerHeight - extraH

			extra := m.renderExtraInfor(metricsWidth, extraH, m.theme)
			language := m.renderLanguagesBreakDown(metricsWidth, langH, m.theme)
			metricsAndLanguages := lipgloss.JoinVertical(lipgloss.Left, extra, language)
			detailSection := lipgloss.JoinHorizontal(lipgloss.Top, fileListRendered, metricsAndLanguages)
			right := lipgloss.JoinVertical(lipgloss.Left, tableRendered, detailSection)

			mainView = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
		} else {
			tableHeight := max(6, min(12, availH*38/100))
			lowerHeight := availH - tableHeight

			fileListWidth := max(22, min(36, leftWidth*40/100))
			metricsWidth := leftWidth - fileListWidth

			tableRendered := RenderTable(m.table, m.active, leftWidth, tableHeight, m.theme)
			fileListRendered := RenderFileList(m.list, m.active, fileListWidth, lowerHeight, m.theme)

			extraH := 7
			if lowerHeight < 14 {
				extraH = max(4, lowerHeight/2)
			}
			langH := lowerHeight - extraH

			extra := m.renderExtraInfor(metricsWidth, extraH, m.theme)
			language := m.renderLanguagesBreakDown(metricsWidth, langH, m.theme)
			metricsAndLanguages := lipgloss.JoinVertical(lipgloss.Left, extra, language)
			detailSection := lipgloss.JoinHorizontal(lipgloss.Top, fileListRendered, metricsAndLanguages)
			left := lipgloss.JoinVertical(lipgloss.Left, tableRendered, detailSection)

			readmeBorder := lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(m.theme.Border).
				Width(rightWidth - 2).
				Height(availH - 2).
				MaxHeight(availH)

			if m.active == readmeActive {
				readmeBorder = readmeBorder.BorderForeground(m.theme.FocusBorder)
			}
			readmeTitle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render("Readme Preview:")
			readmeContent := lipgloss.JoinVertical(lipgloss.Left, readmeTitle, m.viewport.View())
			right := readmeBorder.Render(readmeContent)

			mainView = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
		}
	} else {
		// Compact Mode (width < 110)
		contentWidth := width
		tableHeight := max(6, min(10, availH*38/100))
		lowerHeight := availH - tableHeight

		if m.active == readmeActive {
			readmeBorder := lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(m.theme.FocusBorder).
				Width(contentWidth - 2).
				Height(availH - 2).
				MaxHeight(availH)
			readmeTitle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render("Readme Preview (Full Screen):")
			readmeContent := lipgloss.JoinVertical(lipgloss.Left, readmeTitle, m.viewport.View())
			mainView = readmeBorder.Render(readmeContent)
		} else {
			fileListWidth := max(22, contentWidth*42/100)
			metricsWidth := contentWidth - fileListWidth

			tableRendered := RenderTable(m.table, m.active, contentWidth, tableHeight, m.theme)
			fileListRendered := RenderFileList(m.list, m.active, fileListWidth, lowerHeight, m.theme)

			extraH := 7
			if lowerHeight < 14 {
				extraH = max(4, lowerHeight/2)
			}
			langH := lowerHeight - extraH

			extra := m.renderExtraInfor(metricsWidth, extraH, m.theme)
			language := m.renderLanguagesBreakDown(metricsWidth, langH, m.theme)
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
		Foreground(m.theme.Background).
		Background(m.theme.Primary).
		Padding(0, 1)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
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

	themePill := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Background(m.theme.Surface).
		Padding(0, 1).
		Render(fmt.Sprintf("🎨 %s [t]", m.theme.ID))

	statusItems := []string{tab1, tab2, tab3, " ", themePill}
	if m.cfg != nil && m.cfg.User != "" {
		userPill := lipgloss.NewStyle().
			Foreground(m.theme.Accent).
			Background(m.theme.Surface).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("👤 @%s", m.cfg.User))
		statusItems = append(statusItems, " ", userPill)
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, statusItems...)

	helpStr := "[Tab] Cycle  [1-3] Focus  [t] Theme  [q] Quit"
	if width < 120 {
		helpStr = "[Tab] Focus  [t] Theme  [q] Quit"
	}
	if width < 95 {
		helpStr = "[Tab] Nav  [q] Quit"
	}
	if width < 70 {
		helpStr = "[q] Quit"
	}

	helpText := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(helpStr)
	gap := width - lipgloss.Width(tabs) - lipgloss.Width(helpText)

	if gap < 1 && width >= 80 {
		helpStr = "[q] Quit"
		helpText = lipgloss.NewStyle().Foreground(m.theme.Muted).Render(helpStr)
		gap = width - lipgloss.Width(tabs) - lipgloss.Width(helpText)
	}

	if gap < 1 {
		statusItems = []string{tab1, tab2, tab3}
		tabs = lipgloss.JoinHorizontal(lipgloss.Top, statusItems...)
		gap = max(1, width-lipgloss.Width(tabs)-lipgloss.Width(helpText))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Center, tabs, strings.Repeat(" ", gap), helpText)
	return lipgloss.NewStyle().
		Width(width).
		Height(1).
		MaxHeight(1).
		Background(m.theme.Background).
		Render(bar)
}

func (m Model) renderThemeSelectorModal(width, height int) string {
	padV := 1
	if height < 17 {
		padV = 0
	}
	modalBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(padV, 2).
		Width(min(46, width-2)).
		MaxWidth(width).
		MaxHeight(height)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary).
		Render("🎨 SELECT THEME")

	subtitle := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("Choose an aesthetic palette")

	var rows []string
	for i, t := range theme.Catalog {
		cursor := "  "
		itemStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
		if i == m.themeIndex {
			cursor = "▶ "
			itemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(m.theme.Background).
				Background(m.theme.Primary)
		}

		check := " "
		if t.ID == m.theme.ID {
			check = "✓"
		}

		colorSwatch := lipgloss.NewStyle().Foreground(t.Primary).Render("●")
		rowText := fmt.Sprintf("%s%s %-18s %s", cursor, colorSwatch, t.Name, check)
		rows = append(rows, itemStyle.Render(rowText))
	}

	themeList := strings.Join(rows, "\n")
	help := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("[↑/↓] Navigate  [Enter] Select  [Esc/t] Close")

	var content string
	if height < 16 {
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			themeList,
			help,
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			subtitle,
			"",
			themeList,
			"",
			help,
		)
	}

	card := modalBorder.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

func RenderWithLoading(cfg *flags.CmdConfig, fetcher func() ([]Repo, error)) (tea.Model, error) {
	loading := newLoadingModel()

	themeName := "catppuccin"
	if cfg != nil && cfg.Theme != "" {
		themeName = cfg.Theme
	} else {
		appCfg := appconfig.LoadConfig()
		if appCfg.Theme != "" {
			themeName = appCfg.Theme
		}
	}
	th := theme.GetTheme(themeName)

	m := Model{
		state:   stateLoading,
		loading: loading,
		fetcher: fetcher,
		cfg:     cfg,
		theme:   th,
		width:   80,
		height:  24,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	return p.Run()
}

func Render(cfg *flags.CmdConfig, repos []Repo) (tea.Model, error) {
	fetcher := func() ([]Repo, error) {
		return repos, nil
	}
	return RenderWithLoading(cfg, fetcher)
}
