package ui

import (
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func RenderTable(t table.Model, state activeComponent, width, height int, th theme.Theme) string {
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.Border)
	if width > 0 {
		style = style.Width(max(10, width-2)).MaxWidth(width)
	}
	if height > 0 {
		style = style.Height(max(1, height-2)).MaxHeight(height)
	}
	if state == tableActive {
		style = style.BorderForeground(th.FocusBorder)
	}
	return style.Render(t.View())
}

func CalculateTableColumns(width int) []table.Column {
	// RoundedBorder takes 2 chars; 5 columns with 2 padding each = 10 chars. Total overhead = 12.
	usable := width - 12
	if usable < 30 {
		usable = 30
	}

	ownerW := max(8, usable*20/100)
	repoW := max(12, usable*36/100)
	starsW := max(5, usable*12/100)
	forksW := max(5, usable*12/100)
	langW := usable - (ownerW + repoW + starsW + forksW)
	if langW < 6 {
		langW = 6
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
		t.SetWidth(max(10, width-2))
	}
	if height > 0 {
		// Table body height = box height - 4 (1 header text + 1 header bottom border + 2 outer box borders)
		t.SetHeight(max(1, height-4))
	}
}

func ApplyThemeToTable(t *table.Model, th theme.Theme) {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(th.Border).
		Foreground(th.Text).
		Background(th.Surface).
		BorderBottom(true).
		Bold(true)

	s.Selected = s.Selected.
		Foreground(th.Background).
		Background(th.Primary).
		Bold(true)

	t.SetStyles(s)
}

func InitialTable(repos []Repo, limit int, th theme.Theme) table.Model {
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
		table.WithHeight(7),
		table.WithWidth(78),
	)

	ApplyThemeToTable(&t, th)
	return t
}
