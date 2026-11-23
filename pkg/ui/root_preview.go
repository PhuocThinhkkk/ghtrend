package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	name string
	meta string
}

func (i item) Title() string       { return i.name }
func (i item) Description() string { return i.meta }
func (i item) FilterValue() string { return i.name }

func (m *Model) setFileList() {
	items := []list.Item{}

	tableCursor := m.table.Cursor()
	repos := m.activeRepoList()
	if tableCursor < 0 || tableCursor >= len(repos) {
		m.list.SetItems(items)
		return
	}

	dirs := repos[tableCursor].RootInfor

	widthAllowed := m.list.Width() - 12
	if widthAllowed < 10 {
		widthAllowed = 25
	}

	for i := 0; i < len(dirs); i++ {
		name := ""
		runes := []rune(dirs[i].Name)
		if len(runes) >= widthAllowed {
			if widthAllowed > 3 {
				name = string(runes[:widthAllowed-3]) + "..."
			} else {
				name = string(runes)
			}
		} else {
			name = dirs[i].Name
		}

		icon := "📄 "
		if dirs[i].Type == "dir" {
			icon = "📁 "
		}

		meta := formatFileSize(dirs[i].Name, dirs[i].Type)

		newItem := item{
			name: icon + name,
			meta: meta,
		}
		items = append(items, newItem)
	}
	m.list.SetItems(items)
	m.list.SetDelegate(simpleDelegate{
		selectedColor: m.theme.Primary,
		mutedColor:    m.theme.Muted,
		listWidth:     m.list.Width(),
	})
}

func formatFileSize(name string, t string) string {
	if t == "dir" {
		return "dir"
	}
	return ""
}

func RenderFileList(m list.Model, state activeComponent, width, height int, th theme.Theme) string {
	if width <= 0 {
		width = 30
	}
	return WrapInTitledBorder(m.View(), width, height, "File Preview", state == listActive, th)
}

func InitialFileList(dirs []EntryInfor) list.Model {
	items := []list.Item{}

	for i := 0; i < len(dirs); i++ {
		icon := "📄 "
		if dirs[i].Type == "dir" {
			icon = "📁 "
		}
		meta := formatFileSize(dirs[i].Name, dirs[i].Type)
		newItem := item{
			name: icon + dirs[i].Name,
			meta: meta,
		}
		items = append(items, newItem)
	}

	m := list.New(items, simpleDelegate{}, 35, 14)
	m.SetShowTitle(false)
	m.SetFilteringEnabled(false)
	m.SetShowHelp(false)
	m.SetShowStatusBar(false)
	m.SetShowPagination(true)

	return m
}

type simpleDelegate struct {
	selectedColor lipgloss.TerminalColor
	mutedColor    lipgloss.TerminalColor
	listWidth     int
}

func (d simpleDelegate) Height() int                               { return 1 }
func (d simpleDelegate) Spacing() int                              { return 0 }
func (d simpleDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d simpleDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	left := i.Title()
	right := i.Description()

	totalW := d.listWidth
	if totalW < 20 {
		totalW = m.Width()
	}
	if totalW < 20 {
		totalW = 32
	}

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	availW := totalW - 4 // account for left/right margins inside border

	if leftW+rightW+1 > availW && availW > rightW+4 {
		maxLeft := availW - rightW - 2
		if lipgloss.Width(left) > maxLeft && maxLeft > 3 {
			runes := []rune(left)
			for len(runes) > 0 && lipgloss.Width(string(runes)+"..") > maxLeft {
				runes = runes[:len(runes)-1]
			}
			left = string(runes) + ".."
			leftW = lipgloss.Width(left)
		}
	}

	var line string
	if right == "" {
		if index == m.Index() {
			col := d.selectedColor
			if col == nil {
				col = lipgloss.Color("#7aa2f7")
			}
			leftStyled := lipgloss.NewStyle().Foreground(col).Bold(true).Render(left)
			line = " " + leftStyled
		} else {
			line = " " + left
		}
	} else {
		gap := availW - leftW - rightW - 1
		if gap < 1 {
			gap = 1
		}

		mutedCol := d.mutedColor
		if mutedCol == nil {
			mutedCol = lipgloss.Color("#565f89")
		}
		rightStyled := lipgloss.NewStyle().Foreground(mutedCol).Render(right)

		if index == m.Index() {
			col := d.selectedColor
			if col == nil {
				col = lipgloss.Color("#7aa2f7")
			}
			leftStyled := lipgloss.NewStyle().Foreground(col).Bold(true).Render(left)
			line = " " + leftStyled + strings.Repeat(" ", gap) + rightStyled
		} else {
			line = " " + left + strings.Repeat(" ", gap) + rightStyled
		}
	}

	fmt.Fprint(w, line)
}
