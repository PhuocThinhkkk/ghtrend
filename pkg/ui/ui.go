package ui

import (
    tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"ghtrend/pkg/types"
	"github.com/charmbracelet/bubbles/table"
)

var debugMode = true

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))


type Model struct {
   	table table.Model
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
	return baseStyle.Render(m.table.View())

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
		table.WithHeight(15),
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

	m := Model{t}
    p := tea.NewProgram(m)
    return p.Run()
}

