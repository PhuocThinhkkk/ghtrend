package ui

import (
	"fmt"
	"io"

	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder(), true).
	BorderForeground(lipgloss.Color("63"))

type item struct {
	name string
}

func (i item) Title() string       { return i.name }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.name }

func (m *Model) setFileList() {
	items := []list.Item{}

	tableCursor := m.table.Cursor()
	if tableCursor < 0 || tableCursor >= len(m.repoList) {
		return
	}
	dirs := m.repoList[tableCursor].RootInfor
	widthAllowed := m.list.Width() - 7
	if widthAllowed <= 0 {
		widthAllowed = 25
	}

	for i := 0; i <= len(dirs)-1; i++ {
		name := ""
		if len(dirs[i].Name) >= widthAllowed {
			if widthAllowed > 3 {
				name = dirs[i].Name[:widthAllowed-3] + "..."
			} else {
				name = dirs[i].Name
			}
		} else {
			name = dirs[i].Name
		}

		var newItem item
		if dirs[i].Type == "dir" {
			newItem = item{
				name: "📁 " + name,
			}
		} else {
			newItem = item{
				name: "📄 " + name,
			}
		}
		items = append(items, newItem)

	}
	m.list.SetItems(items)
}

func RenderFileList(m list.Model, state activeComponent, width, height int, th theme.Theme) string {
	style := docStyle.BorderForeground(th.Border)
	if width > 0 {
		style = style.Width(width)
	}
	if height > 0 {
		style = style.Height(height)
	}
	if state == listActive {
		style = style.BorderForeground(th.FocusBorder)
	}
	return style.Render(m.View())
}

func InitialFileList(dirs []EntryInfor) list.Model {
	items := []list.Item{}

	for i := 0; i <= len(dirs)-1; i++ {
		var newItem item
		if dirs[i].Type == "dir" {
			newItem = item{
				name: "📁 " + dirs[i].Name,
			}
		} else {
			newItem = item{
				name: "📄 " + dirs[i].Name,
			}
		}

		items = append(items, newItem)

	}

	m := list.New(items, simpleDelegate{}, 35, 14)
	m.Title = "File Preview:"
	m.SetFilteringEnabled(false)
	m.SetShowHelp(false)
	m.SetShowStatusBar(false)
	m.SetShowPagination(true)

	return m
}

type simpleDelegate struct{}

func (d simpleDelegate) Height() int                               { return 1 }
func (d simpleDelegate) Spacing() int                              { return 0 }
func (d simpleDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d simpleDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := i.Title()

	if index == m.Index() {
		str = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Render("  " + str)
	} else {
		str = "  " + str
	}

	fmt.Fprint(w, str)
}
