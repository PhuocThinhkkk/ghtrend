package ui

import (
	"log"
    tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"ghtrend/pkg/types"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/list"
)

var debugMode = true

type activeComponent int8
const tableActive = 0
const listActive  = 1

type Model struct {
   	table table.Model
	list  list.Model
	repoList []types.Repo
	viewport  viewport.Model
	active    activeComponent
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
      
        }
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
	}
    m.table, cmd = m.table.Update(msg)		
	return m, cmd
}

func (m Model) View() string {
	table := RenderTable(m.table)
	fileList := RenderFileList(m.list)

	left := lipgloss.JoinVertical(lipgloss.Left, table, fileList)
	readMe, err:= RenderReadme(m.repoList[m.table.Cursor()].ReadMe)
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

func Render(repos []types.Repo) (tea.Model, error) {
	table := InitialTable(repos)
	list := InitialFileList(repos[0].RootInfor)
	m := Model{
		table: table,
		list :  list,
		repoList: repos,
		active: tableActive,
	}
	p := tea.NewProgram(m)
    return p.Run()
}
