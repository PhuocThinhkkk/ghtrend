package ui

import (
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func RenderTable(t table.Model, state activeComponent, width, height int, th theme.Theme) string {
	if width <= 0 {
		width = 40
	}
	return WrapInTitledBorder(t.View(), width, height, "Repositories", state == tableActive, th)
}

func CalculateTableColumns(width int) []table.Column {
	return CalculateTableColumnsForMode(width, false)
}

func CalculateTableColumnsForMode(width int, isUser bool) []table.Column {
	// Account for 2 border characters + 2 padding cells per column (5 columns * 2 = 10 cells)
	usable := width - 12
	if usable < 22 {
		usable = 22
	}

	starsW := max(3, usable*8/100)
	forksW := max(3, usable*8/100)
	langW := max(5, usable*14/100)
	rem := usable - (starsW + forksW + langW)

	if isUser {
		repoW := max(8, rem*45/100)
		descW := rem - repoW
		if descW < 8 {
			descW = 8
			repoW = rem - descW
		}
		return []table.Column{
			{Title: "Repo name", Width: repoW},
			{Title: "Description", Width: descW},
			{Title: "Stars", Width: starsW},
			{Title: "Forks", Width: forksW},
			{Title: "Language", Width: langW},
		}
	}

	ownerW := max(7, rem*35/100)
	repoW := rem - ownerW
	if repoW < 8 {
		repoW = 8
		ownerW = rem - repoW
	}

	return []table.Column{
		{Title: "Owner", Width: ownerW},
		{Title: "Repo name", Width: repoW},
		{Title: "Stars", Width: starsW},
		{Title: "Forks", Width: forksW},
		{Title: "Language", Width: langW},
	}
}

func UpdateTableDimensions(t *table.Model, width, height int, isUser ...bool) {
	userMode := false
	if len(isUser) > 0 {
		userMode = isUser[0]
	}
	if width > 0 {
		cols := CalculateTableColumnsForMode(width, userMode)
		t.SetColumns(cols)
		t.SetWidth(max(10, width-2))
	}
	if height > 0 {
		t.SetHeight(max(3, height-2))
	}
}

func ApplyThemeToTable(t *table.Model, th theme.Theme) {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(th.Border).
		Foreground(th.Muted).
		Background(th.Background).
		BorderBottom(true).
		Bold(true)

	s.Selected = s.Selected.
		Foreground(th.Background).
		Background(th.Primary).
		Bold(true)

	t.SetStyles(s)
}

func InitialTable(repos []Repo, limit int, th theme.Theme) table.Model {
	isUser := len(repos) > 0 && repos[0].UserProfile != nil
	return InitialTableWithMode(repos, limit, th, isUser)
}

func InitialTableWithMode(repos []Repo, limit int, th theme.Theme, isUser bool) table.Model {
	columns := CalculateTableColumnsForMode(80, isUser)
	var rows []table.Row

	for _, repo := range repos {
		if limit == 0 {
			break
		}
		displayName := repo.Name
		if repo.Pinned {
			displayName = "📌 " + repo.Name
		}

		stars := repo.Stars
		if stars == "" || stars == "0" {
			stars = "·"
		}
		forks := repo.Forks
		if forks == "" || forks == "0" {
			forks = "·"
		}

		if isUser {
			desc := repo.Description
			if desc == "" {
				desc = "·"
			}
			rows = append(rows, table.Row{
				displayName,
				desc,
				stars,
				forks,
				repo.Language,
			})
		} else {
			rows = append(rows, table.Row{
				repo.Owner,
				displayName,
				stars,
				forks,
				repo.Language,
			})
		}
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
