package ui

import (
	"github.com/charmbracelet/lipgloss"
	"ghtrend/pkg/types"
	"github.com/charmbracelet/bubbles/table"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4"))


func RenderTable(t table.Model) string {
	table := baseStyle.Render(t.View())
	return table
	
}

func InitialTable(repos []types.Repo) table.Model{

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
		table.WithHeight(12),
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
	return t

}
