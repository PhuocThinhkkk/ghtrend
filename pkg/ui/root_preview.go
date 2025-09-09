package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)
var docStyle = lipgloss.NewStyle().
	Width(35).
    Border(lipgloss.NormalBorder(), true).
    BorderForeground(lipgloss.Color("63")) 

type item struct {
	name string
}

func (i item) Title() string       { return i.name }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.name }

func (m *Model) setFileList() {
	// Bug: new files but the len still be the len of repos[0]
	items := []list.Item{}
	
	tableCursor := m.table.Cursor()
	dirs := m.repoList[tableCursor].RootInfor
	widthAllowed := docStyle.GetWidth() - 7

	for i := 0; i <= len(dirs) - 1 ; i++ {
		name := ""
		if len(dirs[i].Name) >= widthAllowed {
			name = dirs[i].Name[:widthAllowed-3] + "..."
		}else {
			name = dirs[i].Name
		}
			
		var newItem item
		if dirs[i].Type == "dir" {
			newItem = item {
				name: "📁 " + name,
			}
		}else {
			newItem = item {
				name: name,
			}
		}
		items = append(items, newItem)

	}
	 m.list.SetItems(items)
}

func RenderFileList(m list.Model, state activeComponent) string {
	style := docStyle
	if state == listActive {
		style = style.BorderForeground(lipgloss.Color("#BA8E23"))
	}
	return style.Render(m.View())
}

func InitialFileList(dirs []EntryInfor) list.Model{
	items := []list.Item{}

	for i := 0; i <= len(dirs) - 1 ; i++ {
		var newItem item
		if dirs[i].Type == "dir" {
			newItem = item {
				name: "📁 " + dirs[i].Name,
			}
		}else {
			newItem = item {
				name: dirs[i].Name,
			}
		}

		items = append(items, newItem)

	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
	Foreground(lipgloss.Color("229")). 
	Background(lipgloss.Color("57")).   
	Height(1)

	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
	Foreground(lipgloss.Color("229"))

	m :=  list.New(items, simpleDelegate{}, 20, 17)
	m.Title = "File Preview: "
	m.SetFilteringEnabled(false)
	m.SetShowHelp(false)
	m.SetShowStatusBar(false)
	m.SetShowPagination(true)

	return m
}




type simpleDelegate struct{}

func (d simpleDelegate) Height() int                           { return 1 }
func (d simpleDelegate) Spacing() int                          { return 0 }
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

