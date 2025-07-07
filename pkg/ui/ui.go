package ui

import (
    tea "github.com/charmbracelet/bubbletea"
    "fmt"
    "ghtrend/pkg/utils"
)

type Model struct {
    Repos  []utils.Repo
    cursor int
}

func (m Model) Init() tea.Cmd {
    return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.Repos)-1 {
                m.cursor++
            }
        }
    }
    return m, nil
}

func (m Model) View() string {
    s := "Trending github repos: \n\n"
    for i, repo := range m.Repos {
        cursor := " "
        if m.cursor == i {
            cursor = ">"
        }
        s += fmt.Sprintf(" %s %s %s - %s\n",
            cursor,
            repo.Name,
            repo.Stars,
            repo.Language,
        )
    }

    if len(m.Repos) > 0 && m.cursor < len(m.Repos) {
        s += fmt.Sprintf("\n    %s\n", m.Repos[m.cursor].Description)
    }

    s += "\n[Press ↑/↓ to navigate, q to quit]"
    return s
}

func Render(repos []utils.Repo) (tea.Model, error) {
    m := Model{
        Repos:  repos,
        cursor: 0,
    }
    p := tea.NewProgram(m)
    return p.Run()
}

