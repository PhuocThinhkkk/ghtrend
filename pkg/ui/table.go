package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/table"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4"))


func RenderTable(t table.Model) string {
	table := baseStyle.Render(t.View())
	return table
	
}

func InitialTable(repos []Repo) table.Model{

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
		table.WithWidth(80),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("60")).
	Foreground(lipgloss.Color("183")).    
	Background(lipgloss.Color("54")).    
	BorderBottom(true).
	Bold(true)

	s.Selected = s.Selected.
	Foreground(lipgloss.Color("16")).   
	Background(lipgloss.Color("99")).  
	Bold(true)

	t.SetStyles(s)
	fmt.Println("hi mom",t.Width())
	return t

}
