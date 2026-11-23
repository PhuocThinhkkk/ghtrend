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
	"github.com/charmbracelet/bubbles/textinput"
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
	showHelpModal  bool
	themeIndex     int
	table          table.Model
	list           list.Model
	repoList       []Repo
	filteredRepos  []Repo
	viewport       viewport.Model
	active         activeComponent
	width          int
	height         int
	err            error
	userProfile    *ghclient.UserProfile
	filtering      bool
	filterInput    textinput.Model
	toastMessage   string
	toastID        int
}

func (m *Model) activeRepoList() []Repo {
	if m.filteredRepos != nil {
		return m.filteredRepos
	}
	return m.repoList
}

func (m *Model) getCursorRepo() Repo {
	repos := m.activeRepoList()
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(repos) {
		if len(repos) > 0 {
			return repos[0]
		}
		return Repo{}
	}
	return repos[cursor]
}

type clearToastMsg struct {
	id int
}

func clearToastCmd(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearToastMsg{id: id}
	})
}

func (m *Model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.filteredRepos = m.repoList
	} else {
		var matched []Repo
		for _, r := range m.repoList {
			nameMatch := strings.Contains(strings.ToLower(r.Name), query)
			ownerMatch := strings.Contains(strings.ToLower(r.Owner), query)
			langMatch := strings.Contains(strings.ToLower(r.Language), query)
			descMatch := strings.Contains(strings.ToLower(r.Description), query)
			if nameMatch || ownerMatch || langMatch || descMatch {
				matched = append(matched, r)
			}
		}
		m.filteredRepos = matched
	}

	limit := len(m.filteredRepos)
	if m.cfg != nil && m.cfg.Limit > 0 && m.cfg.Limit < limit {
		limit = m.cfg.Limit
	}
	m.table = InitialTable(m.filteredRepos, limit, m.theme)
	m.recalculateLayout(m.width, m.height)
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
	case clearToastMsg:
		if msg.id == m.toastID {
			m.toastMessage = ""
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

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

		if m.showHelpModal {
			switch msg.String() {
			case "esc", "?", "h", "q", "enter":
				m.showHelpModal = false
				return m, nil
			}
			return m, nil
		}

		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterInput.Reset()
				m.filterInput.Blur()
				m.applyFilter()
				return m, nil
			case "enter":
				m.filtering = false
				m.filterInput.Blur()
				return m, nil
			default:
				var tiCmd tea.Cmd
				m.filterInput, tiCmd = m.filterInput.Update(msg)
				m.applyFilter()
				return m, tiCmd
			}
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			if m.state == stateDashboard && m.filterInput.Value() != "" {
				m.filterInput.Reset()
				m.filterInput.Blur()
				m.applyFilter()
				return m, nil
			}
		case "/":
			if m.state == stateDashboard {
				m.filtering = true
				m.filterInput.Focus()
				return m, textinput.Blink
			}
		case "o":
			if m.state == stateDashboard {
				repo := m.getCursorRepo()
				if repo.Url != "" {
					m.toastID++
					id := m.toastID
					if err := utils.OpenBrowser(repo.Url); err != nil {
						m.toastMessage = "❌ Failed to open browser"
					} else {
						repoName := repo.Name
						if len(repoName) > 25 {
							repoName = repoName[:22] + "..."
						}
						m.toastMessage = fmt.Sprintf("🌐 Opened %s", repoName)
					}
					return m, clearToastCmd(id, 2*time.Second)
				}
			}
		case "y":
			if m.state == stateDashboard {
				repo := m.getCursorRepo()
				if repo.Url != "" {
					m.toastID++
					id := m.toastID
					if err := utils.CopyToClipboard(repo.Url); err != nil {
						m.toastMessage = "❌ Failed to copy to clipboard"
					} else {
						m.toastMessage = "📋 Copied URL!"
					}
					return m, clearToastCmd(id, 2*time.Second)
				}
			}
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
		case "d", "f":
			if m.state == stateDashboard {
				if m.cfg != nil && m.cfg.User != "" {
					m.toastID++
					id := m.toastID
					m.toastMessage = "ℹ Frequency applies to trending mode"
					return m, clearToastCmd(id, 2*time.Second)
				}
				if m.cfg != nil {
					m.cfg.Since = flags.NextFrequency(m.cfg.Since)
					m.toastID++
					id := m.toastID
					m.toastMessage = fmt.Sprintf("📅 %s Trending", m.cfg.Since.DisplayTitle())
					m.state = stateLoading
					m.loading = newLoadingModel()
					return m, tea.Batch(m.loading.Init(), fetchReposCmd(m.fetcher), clearToastCmd(id, 2*time.Second))
				}
			}
		case "?", "h":
			if m.state == stateDashboard {
				m.showHelpModal = true
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

		ti := textinput.New()
		ti.Placeholder = "type to filter..."
		ti.Prompt = ""
		ti.CharLimit = 50
		ti.Width = 30
		m.filterInput = ti
		m.filteredRepos = msg.repos
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

	if m.filtering {
		var tiCmd tea.Cmd
		m.filterInput, tiCmd = m.filterInput.Update(msg)
		return m, tiCmd
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
	repos := m.activeRepoList()
	if len(repos) == 0 {
		m.viewport.SetContent("_No repositories match the current filter._")
		return
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(repos) {
		return
	}

	rawMD := repos[cursor].ReadMe
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

	if m.showHelpModal {
		return m.renderHelpModal(width, height)
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
	if m.filtering {
		filterPrompt := lipgloss.NewStyle().Foreground(m.theme.Primary).Bold(true).Render("🔍 Filter: ")
		filterInputView := m.filterInput.View()
		filterHelp := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("[Enter] Apply  [Esc] Clear")
		leftPart := lipgloss.JoinHorizontal(lipgloss.Left, filterPrompt, filterInputView)
		gap := width - lipgloss.Width(leftPart) - lipgloss.Width(filterHelp)
		if gap < 1 {
			filterHelp = lipgloss.NewStyle().Foreground(m.theme.Muted).Render("[Enter]")
			gap = width - lipgloss.Width(leftPart) - lipgloss.Width(filterHelp)
		}
		if gap < 1 {
			filterHelp = ""
			gap = max(1, width-lipgloss.Width(leftPart))
		}
		bar := lipgloss.JoinHorizontal(lipgloss.Center, leftPart, strings.Repeat(" ", max(1, gap)), filterHelp)
		return lipgloss.NewStyle().
			Width(width).
			Height(1).
			MaxHeight(1).
			Background(m.theme.Background).
			Render(bar)
	}

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

	var userPill string
	if m.cfg != nil && m.cfg.User != "" {
		userPill = lipgloss.NewStyle().
			Foreground(m.theme.Accent).
			Background(m.theme.Surface).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("👤 @%s", m.cfg.User))
	}

	var freqPill string
	if m.cfg != nil && m.cfg.User == "" {
		freqPill = lipgloss.NewStyle().
			Foreground(m.theme.Primary).
			Background(m.theme.Surface).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("📅 %s [d]", m.cfg.Since.DisplayTitle()))
	}

	var toastPill string
	if m.toastMessage != "" {
		toastPill = lipgloss.NewStyle().
			Foreground(m.theme.Background).
			Background(m.theme.Accent).
			Bold(true).
			Padding(0, 1).
			Render(m.toastMessage)
	}

	// Build full status item list in priority order: Tabs > Toast > Frequency > User > Theme
	statusItems := []string{tab1, tab2, tab3}
	if toastPill != "" {
		statusItems = append(statusItems, " ", toastPill)
	}
	if freqPill != "" {
		statusItems = append(statusItems, " ", freqPill)
	}
	if userPill != "" {
		statusItems = append(statusItems, " ", userPill)
	}
	statusItems = append(statusItems, " ", themePill)

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, statusItems...)

	helpStr := "[/] Filter  [d] Freq  [o] Open  [y] Copy  [t] Theme  [?] Help  [q] Quit"
	if width < 145 {
		helpStr = "[/] Filter  [d] Freq  [o] Open  [y] Copy  [t] Theme  [?] Help  [q] Quit"
	}
	if width < 125 {
		helpStr = "[/] Filter  [d] Freq  [o] Open  [t] Theme  [?] Help  [q] Quit"
	}
	if width < 105 {
		helpStr = "[/] Filter  [d] Freq  [o] Open  [?] Help  [q] Quit"
	}
	if width < 85 {
		helpStr = "[Tab] Nav  [?] Help  [q] Quit"
	}
	if width < 70 {
		helpStr = "[?] Help  [q] Quit"
	}

	helpText := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(helpStr)
	gap := width - lipgloss.Width(tabs) - lipgloss.Width(helpText)

	// Step 1: if too tight, simplify help string first
	if gap < 1 && width >= 80 {
		helpStr = "[?] Help  [q] Quit"
		helpText = lipgloss.NewStyle().Foreground(m.theme.Muted).Render(helpStr)
		gap = width - lipgloss.Width(tabs) - lipgloss.Width(helpText)
	}

	// Step 2: if still tight, drop theme and user pills, keeping tabs, toast, and freqPill
	if gap < 1 {
		statusItems = []string{tab1, tab2, tab3}
		if toastPill != "" {
			statusItems = append(statusItems, " ", toastPill)
		}
		if freqPill != "" {
			statusItems = append(statusItems, " ", freqPill)
		}
		tabs = lipgloss.JoinHorizontal(lipgloss.Top, statusItems...)
		gap = width - lipgloss.Width(tabs) - lipgloss.Width(helpText)
	}

	// Step 3: if still tight, drop freqPill too, keeping tabs and toast
	if gap < 1 {
		statusItems = []string{tab1, tab2, tab3}
		if toastPill != "" {
			statusItems = append(statusItems, " ", toastPill)
		}
		tabs = lipgloss.JoinHorizontal(lipgloss.Top, statusItems...)
		gap = width - lipgloss.Width(tabs) - lipgloss.Width(helpText)
	}

	// Step 4: if still tight on very narrow terminals (<65), drop toast
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

func (m Model) renderHelpModal(width, height int) string {
	padV := 1
	if height < 22 {
		padV = 0
	}
	modalBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(padV, 2).
		Width(min(64, width-2)).
		MaxWidth(width).
		MaxHeight(height)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary).
		Render("📖 KEYBOARD SHORTCUTS")

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Secondary)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Accent)

	descStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text)

	formatRow := func(key, desc string) string {
		return fmt.Sprintf("  %-18s %s", keyStyle.Render(key), descStyle.Render(desc))
	}

	var sections []string
	sections = append(sections, title)

	if height >= 24 {
		sections = append(sections,
			"",
			sectionStyle.Render("NAVIGATION"),
			formatRow("Tab / Shift+Tab", "Cycle focus between panes"),
			formatRow("1, 2, 3", "Jump to Table, Files, Readme"),
			formatRow("↑/↓, k/j", "Navigate rows or scroll"),
			"",
			sectionStyle.Render("TRENDING & ACTIONS"),
			formatRow("/", "Interactive in-place filter"),
			formatRow("d / f", "Cycle Daily ⇄ Weekly ⇄ Monthly"),
			formatRow("o", "Open repository in browser"),
			formatRow("y", "Copy repository URL"),
			"",
			sectionStyle.Render("SETTINGS & SYSTEM"),
			formatRow("t", "Color theme palette selector"),
			formatRow("? / h", "Toggle this help shortcut dialog"),
			formatRow("Esc", "Reset search / close modals"),
			formatRow("q / Ctrl+C", "Quit application"),
		)
	} else {
		sections = append(sections,
			formatRow("Tab / 1-3", "Cycle or jump panes"),
			formatRow("↑/↓ / k/j", "Navigate items"),
			formatRow("/", "Fuzzy search filter"),
			formatRow("d / f", "Cycle Daily/Weekly/Monthly"),
			formatRow("o / y", "Open browser / Copy URL"),
			formatRow("t", "Theme selector"),
			formatRow("? / Esc", "Close help / Clear filter"),
			formatRow("q", "Quit"),
		)
	}

	closeHint := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("[Esc] [?] [q] Close")

	sections = append(sections, "", closeHint)
	content := lipgloss.JoinVertical(lipgloss.Center, sections...)
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
