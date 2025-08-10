package ui

import (
	"ghtrend/pkg/types"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2).Height(20).Border(lipgloss.Border{})

type item struct {
	name string
}

func (i item) Title() string       { return i.name }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.name }

func RenderFileList(m list.Model) string {
	return docStyle.Render(m.View())
}

func InitialFileList(dirs []types.EntryInfor) list.Model{
	items := []list.Item{}

	for i := 0; i <= len(dirs) - 1 ; i++ {
		newItem := item {
			name: dirs[i].Name,
		}
		items = append(items, newItem)

	}

	m :=  list.New(items, list.NewDefaultDelegate(), 20, 30)
	m.Title = "My Fave Things"
	return m
}
