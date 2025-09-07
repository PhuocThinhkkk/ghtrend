package ui

import (
	"ghtrend/pkg/configs/flags"
	"ghtrend/pkg/ghclient"
	"log"
	"os"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var debugMode = true

type activeComponent int8

const tableActive = 0
const listActive = 1

type Repo = ghclient.Repo
type EntryInfor = ghclient.EntryInfor

type Model struct {
	table    table.Model
	list     list.Model
	repoList []Repo
	viewport viewport.Model
	active   activeComponent
}

func (m *Model) getCursorRepo() Repo {
	return m.repoList[m.table.Cursor()]
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
			if m.active == tableActive {
				m.active = listActive
			} else if m.active == listActive {
				m.active = tableActive
			}

		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
	}

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
	}
	return m, cmd
}

func (m Model) View() string {
	table := RenderTable(m.table)
	m.setFileList()
	fileList := RenderFileList(m.list)
	extra := m.renderExtraInfor()
	language := m.renderLanguagesBreakDown()
	poop := lipgloss.JoinVertical(lipgloss.Left, extra, language)
	shit := lipgloss.JoinHorizontal(lipgloss.Top, fileList, poop)

	left := lipgloss.JoinVertical(lipgloss.Left, table, shit)
	readMe, err := RenderReadme(m.repoList[m.table.Cursor()].ReadMe)
	if err != nil {
		log.Fatal("Error when render readme markdown: ", err)
	}

	view := lipgloss.JoinHorizontal(lipgloss.Top, left, readMe)
	content := lipgloss.NewStyle().
		Width(m.viewport.Width).
		Height(m.viewport.Height).
		Render(view)

	return content
}

func Render(cfg *flags.CmdConfig, repos []Repo) (tea.Model, error) {
	table := InitialTable(repos, cfg.Limit)
	if len(repos) == 0 {
		fmt.Println("There was no repos match that")
		os.Exit(0)
	}

	list := InitialFileList(repos[0].RootInfor)
	m := Model{
		table:    table,
		list:     list,
		repoList: repos,
		active:   tableActive,
	}
	p := tea.NewProgram(m)
	return p.Run()
}
