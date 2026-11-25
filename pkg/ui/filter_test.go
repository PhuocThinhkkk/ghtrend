package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
)

func TestModel_FilterWorkflow(t *testing.T) {
	repos := []Repo{
		{
			Owner:       "charmbracelet",
			Name:        "bubbletea",
			Language:    "Go",
			Description: "A powerful TUI framework",
		},
		{
			Owner:       "charmbracelet",
			Name:        "lipgloss",
			Language:    "Go",
			Description: "Style definitions for nice terminal UIs",
		},
		{
			Owner:       "torvalds",
			Name:        "linux",
			Language:    "C",
			Description: "Linux kernel source tree",
		},
	}

	th := theme.Catalog[0]
	tbl := InitialTable(repos, 10, th)
	lst := InitialFileList(nil)
	vp := viewport.New(60, 20)

	ti := textinput.New()
	ti.Placeholder = "type to filter..."

	m := Model{
		state:         stateDashboard,
		theme:         th,
		table:         tbl,
		list:          lst,
		repoList:      repos,
		filteredRepos: repos,
		viewport:      vp,
		active:        tableActive,
		width:         120,
		height:        24,
		filterInput:   ti,
	}
	m.recalculateLayout(120, 24)

	// 1. Press '/' to activate filtering
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newM.(Model)
	if !m.filtering {
		t.Errorf("expected m.filtering to be true after pressing '/'")
	}
	if !m.filterInput.Focused() {
		t.Errorf("expected filterInput to be focused")
	}
	if cmd == nil {
		t.Errorf("expected blink cmd after activating filter")
	}

	// 2. Type 'linux'
	for _, r := range "linux" {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(Model)
	}
	if len(m.filteredRepos) != 1 {
		t.Fatalf("expected 1 filtered repo for query 'linux', got %d", len(m.filteredRepos))
	}
	if m.filteredRepos[0].Name != "linux" {
		t.Errorf("expected filtered repo 'linux', got %q", m.filteredRepos[0].Name)
	}

	// 3. Press Enter to lock filter
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.filtering {
		t.Errorf("expected m.filtering to be false after enter")
	}
	if m.filterInput.Focused() {
		t.Errorf("expected filterInput to be blurred after enter")
	}
	if len(m.filteredRepos) != 1 {
		t.Errorf("expected filter to remain active after enter")
	}

	// 4. Press Esc from dashboard mode to reset filter
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(Model)
	if len(m.filteredRepos) != 3 {
		t.Errorf("expected filter to reset to all 3 repos after pressing esc in dashboard, got %d", len(m.filteredRepos))
	}
	if m.filterInput.Value() != "" {
		t.Errorf("expected filter input to be empty, got %q", m.filterInput.Value())
	}

	// 4b. Zero-match filter must NOT fall back to unfiltered repoList
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newM.(Model)
	for _, r := range "zzzznomatch" {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(Model)
	}
	if got := len(m.activeRepoList()); got != 0 {
		t.Fatalf("expected activeRepoList() length 0 for unmatched query, got %d", got)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(Model)

	// 5. Press '/' then 'esc' to cancel filter while typing
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newM.(Model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(Model)
	if m.filtering {
		t.Errorf("expected m.filtering to be false after esc")
	}
	if m.filterInput.Focused() {
		t.Errorf("expected filterInput to be blurred after esc")
	}
	if len(m.filteredRepos) != 3 {
		t.Errorf("expected filter to reset to all 3 repos, got %d", len(m.filteredRepos))
	}
}

func TestModel_QuickActions_Toast(t *testing.T) {
	repos := []Repo{
		{
			Owner:    "test",
			Name:     "a-very-long-repository-name-that-should-be-truncated-in-toast",
			Language: "Go",
			Url:      "https://github.com/test/repo",
		},
	}

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
		filteredRepos: repos,
		viewport:      vp,
		active:        tableActive,
		width:         120,
		height:        24,
	}
	m.recalculateLayout(120, 24)

	// Press 'y' to copy URL
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newM.(Model)
	if m.toastMessage == "" {
		t.Errorf("expected toastMessage after pressing 'y'")
	}
	if cmd == nil {
		t.Errorf("expected timer cmd after pressing 'y'")
	}

	// Verify view height with toast never exceeds bounds
	v := m.View()
	if lipgloss.Height(v) > 24 {
		t.Errorf("view height %d exceeds 24 with toast", lipgloss.Height(v))
	}

	// Test status bar on constrained width 75 with toast: toast should still render
	m.width = 75
	m.height = 24
	m.recalculateLayout(75, 24)
	bar := m.renderStatusBar(75)
	if lipgloss.Height(bar) > 1 {
		t.Errorf("status bar height %d exceeds 1 on width 75", lipgloss.Height(bar))
	}

	// Deliver clearToastMsg with matching ID
	newM, _ = m.Update(clearToastMsg{id: m.toastID})
	m = newM.(Model)
	if m.toastMessage != "" {
		t.Errorf("expected toastMessage to be cleared, got %q", m.toastMessage)
	}
}
