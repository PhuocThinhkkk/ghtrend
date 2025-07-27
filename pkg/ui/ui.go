package ui

import (
    tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"io"
	"errors"
	"ghtrend/pkg/utils"
	"ghtrend/pkg/types"
	"github.com/charmbracelet/bubbles/table"
	"strings"
	"github.com/charmbracelet/glamour"
)

var debugMode = true

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4")).
	Height(20)

// TODO: clean this later
var (
		borderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(70).
		Height(30)

		headerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5CE1E6")).
		Bold(true)

	)

type Model struct {
   	table table.Model
	repoList []types.Repo
	
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
    }
    m.table, cmd = m.table.Update(msg)		
	return m, cmd
}

func (m Model) View() string {
	left := baseStyle.Render(m.table.View())
	cursor := m.table.Cursor()
	header := headerStyle.Render("Readme Preview: ")

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(60),
		glamour.WithPreservedNewLines(),
	)

	markdown := m.repoList[cursor].ReadMe
	if len(strings.TrimSpace(markdown)) == 0 {
		markdown = "_No README found._"
	} else {
		markdown = utils.CleanMarkdown(markdown)

		maxLines := 22
		lines := strings.Split(markdown, "\n")
		inx := 0
		countLine := 0
		for countLine <= maxLines {
			countLine += len(lines[inx])/60 + 1
			inx++
		}
		lines = append(lines[:(inx - 1)], "...") // add ellipsis
		markdown = strings.Join(lines, "\n")

	}
	

	body, err := renderer.Render(markdown)
	if err != nil && !errors.Is(err, io.EOF) {
		panic(err)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	right := borderStyle.Render(content)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func Render(repos []types.Repo) (tea.Model, error) {
   	columns := []table.Column{
		{Title: "Owner", Width: 15 },
		{Title: "Repo name", Width: 25},
		{Title: "Stars ", Width: 10},
		{Title: "Forks", Width: 10},
		{Title: "Language", Width: 15},
	}
	var rows []table.Row

	for _, repo := range repos {
		rows = append(rows, table.Row{
			repo.Owner,
			repo.Name,
			repo.Stars,
			repo.Forks,
			repo.Language,
		})
	}


		t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("60")).     // muted purple border
	Foreground(lipgloss.Color("183")).          // soft lavender text
	Background(lipgloss.Color("54")).           // dark purple background
	BorderBottom(true).
	Bold(true)

	s.Selected = s.Selected.
	Foreground(lipgloss.Color("16")).           // near-black text
	Background(lipgloss.Color("99")).           // hot pink-purple highlight
	Bold(true)
	t.SetStyles(s)

	m := Model{
		table: t,
		repoList: repos,
	}
	p := tea.NewProgram(m)
    return p.Run()
}
