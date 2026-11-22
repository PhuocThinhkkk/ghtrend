package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4"))


func RenderTable(t table.Model, state activeComponent, width int) string {
	style := baseStyle
	if width > 0 {
		style = style.Width(width)
	}
	if state == tableActive {
		style = style.BorderForeground(lipgloss.Color("#BA8E23"))
	}
	return style.Render(t.View())
}

func CalculateTableColumns(width int) []table.Column {
	usable := width - 6
	if usable < 40 {
		usable = 40
	}

	ownerW := max(10, usable*20/100)
	repoW := max(16, usable*36/100)
	starsW := max(6, usable*12/100)
	forksW := max(6, usable*12/100)
	langW := usable - (ownerW + repoW + starsW + forksW)
	if langW < 8 {
		langW = 8
	}

	return []table.Column{
		{Title: "Owner", Width: ownerW},
		{Title: "Repo name", Width: repoW},
		{Title: "Stars", Width: starsW},
		{Title: "Forks", Width: forksW},
		{Title: "Language", Width: langW},
	}
}

func UpdateTableDimensions(t *table.Model, width, height int) {
	if width > 0 {
		cols := CalculateTableColumns(width)
		t.SetColumns(cols)
		t.SetWidth(width)
	}
	if height > 0 {
		t.SetHeight(height)
	}
}

func InitialTable(repos []Repo, limit int) table.Model {
	columns := CalculateTableColumns(80)
	var rows []table.Row

	for _, repo := range repos {
		if limit == 0 {
			break
		}
		rows = append(rows, table.Row{
			repo.Owner,
			repo.Name,
			repo.Stars,
			repo.Forks,
			repo.Language,
		})
		limit--
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
	return t

}
