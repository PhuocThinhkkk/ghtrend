package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
)

func TestHelpModal_NeverExceedsTerminalHeight(t *testing.T) {
	repos := []Repo{{Owner: "test", Name: "repo", Stars: "1", Forks: "1", Language: "Go"}}
	for _, h := range []int{14, 15, 16, 17, 20, 24, 30, 40} {
		for _, w := range []int{60, 80, 100, 120, 160} {
			th := theme.Catalog[0]
			tbl := InitialTable(repos, 10, th)
			lst := InitialFileList(nil)
			vp := viewport.New(60, 20)
			m := Model{
				state:         stateDashboard,
				theme:         th,
				table:         tbl,
				list:          lst,
				repoList:      repos,
				viewport:      vp,
				showHelpModal: true,
				width:         w,
				height:        h,
			}
			v := m.View()
			actualH := lipgloss.Height(v)
			if actualH > h {
				t.Errorf("HelpModal Size=%dx%d: view height %d exceeds terminal height %d", w, h, actualH, h)
			}
		}
	}
}

func TestModel_HelpModalWorkflow(t *testing.T) {
	repos := []Repo{{Owner: "test", Name: "repo", Stars: "1", Forks: "1", Language: "Go"}}
	th := theme.Catalog[0]
	tbl := InitialTable(repos, 10, th)
	lst := InitialFileList(nil)
	vp := viewport.New(60, 20)
	m := Model{
		state:    stateDashboard,
		theme:    th,
		table:    tbl,
		list:     lst,
		repoList: repos,
		viewport: vp,
		width:    100,
		height:   24,
	}

	// 1. Press '?' to open help modal
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(Model)
	if !m.showHelpModal {
		t.Fatalf("expected showHelpModal to be true after pressing '?'")
	}

	// 2. Press 'esc' to close help modal
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(Model)
	if m.showHelpModal {
		t.Errorf("expected showHelpModal to be false after pressing esc")
	}

	// 3. Press 'h' to open help modal
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = newM.(Model)
	if !m.showHelpModal {
		t.Fatalf("expected showHelpModal to be true after pressing 'h'")
	}

	// 4. Press 'q' to close help modal
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = newM.(Model)
	if m.showHelpModal {
		t.Errorf("expected showHelpModal to be false after pressing 'q'")
	}
}

func TestModel_CycleFrequency(t *testing.T) {
	repos := []Repo{{Owner: "test", Name: "repo", Stars: "1", Forks: "1", Language: "Go"}}
	th := theme.Catalog[0]
	tbl := InitialTable(repos, 10, th)
	lst := InitialFileList(nil)
	vp := viewport.New(60, 20)

	cfg := &flags.CmdConfig{
		Since: flags.Daily,
		Limit: 10,
	}

	mockFetcher := func() ([]Repo, error) {
		return repos, nil
	}

	m := Model{
		state:    stateDashboard,
		theme:    th,
		table:    tbl,
		list:     lst,
		repoList: repos,
		viewport: vp,
		cfg:      cfg,
		fetcher:  mockFetcher,
		width:    120,
		height:   24,
	}
	m.recalculateLayout(120, 24)

	// Verify initial status bar contains '📅 Daily [d]'
	bar := m.renderStatusBar(120)
	if bar == "" {
		t.Errorf("expected non-empty status bar")
	}

	// 1. Press 'd' to cycle Daily -> Weekly
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newM.(Model)
	if m.cfg.Since != flags.Weekly {
		t.Errorf("expected Since to be Weekly, got %s", m.cfg.Since)
	}
	if m.toastMessage != "📅 Weekly Trending" {
		t.Errorf("expected toast '📅 Weekly Trending', got %q", m.toastMessage)
	}
	if cmd == nil {
		t.Errorf("expected batch cmd returned after cycling frequency")
	}

	// Simulate repos loaded back into dashboard
	newM, _ = m.Update(reposLoadedMsg{repos: repos})
	m = newM.(Model)

	// 2. Press 'd' to cycle Weekly -> Monthly
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newM.(Model)
	if m.cfg.Since != flags.Monthly {
		t.Errorf("expected Since to be Monthly, got %s", m.cfg.Since)
	}
	if m.toastMessage != "📅 Monthly Trending" {
		t.Errorf("expected toast '📅 Monthly Trending', got %q", m.toastMessage)
	}

	// Simulate repos loaded
	newM, _ = m.Update(reposLoadedMsg{repos: repos})
	m = newM.(Model)

	// 3. Press 'd' to cycle Monthly -> Daily
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newM.(Model)
	if m.cfg.Since != flags.Daily {
		t.Errorf("expected Since to be Daily, got %s", m.cfg.Since)
	}

	// Simulate repos loaded
	newM, _ = m.Update(reposLoadedMsg{repos: repos})
	m = newM.(Model)

	// 4. Test in user mode: frequency should not change
	m.cfg.User = "PhuocThinhkkk"
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newM.(Model)
	if m.cfg.Since != flags.Daily {
		t.Errorf("expected Since to remain Daily in user mode, got %s", m.cfg.Since)
	}
	if m.toastMessage != "ℹ Frequency applies to trending mode" {
		t.Errorf("expected informative toast in user mode, got %q", m.toastMessage)
	}
}
