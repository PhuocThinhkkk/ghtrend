package ui

import (
	"strings"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestCalculateTableColumns(t *testing.T) {
	widths := []int{40, 80, 120, 160, 240}

	for _, w := range widths {
		cols := CalculateTableColumns(w)
		if len(cols) != 5 {
			t.Fatalf("expected 5 columns for width %d, got %d", w, len(cols))
		}

		totalColWidth := 0
		for _, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column %q has non-positive width %d for total width %d", col.Title, col.Width, w)
			}
			totalColWidth += col.Width
		}

		if totalColWidth <= 0 {
			t.Errorf("total column width was %d for width %d", totalColWidth, w)
		}
	}
}

func TestRenderEachField_SafePadding(t *testing.T) {
	th := theme.Catalog[0]
	// Must not panic even when totalWidth is smaller than label + value
	out := renderEachField(10, "A Very Long Label Name", "123456789")
	if !strings.Contains(out, "A Very Long Label Name") || !strings.Contains(out, "123456789") {
		t.Errorf("expected output to contain label and value, got %q", out)
	}

	// Normal case
	normal := renderEachField(50, "Commits", "100")
	if !strings.Contains(normal, "Commits") || !strings.Contains(normal, "100") {
		t.Errorf("expected output to contain label and value, got %q", normal)
	}

	header := renderHeader(50, "Metrics", th)
	if !strings.Contains(header, "Metrics") {
		t.Errorf("expected header to contain Metrics, got %q", header)
	}
}

func TestModel_RecalculateLayout(t *testing.T) {
	repos := []Repo{
		{
			Owner:       "google",
			Name:        "go-github",
			Stars:       "12.5k",
			Forks:       "3.2k",
			Language:    "Go",
			Description: "Go library for GitHub API",
			ReadMe:      "# Go GitHub\nA Go library for accessing GitHub API.",
			RootInfor: []EntryInfor{
				{Type: "dir", Name: "github"},
				{Type: "file", Name: "README.md"},
			},
			ExtraInfor: ExtraInfor{
				TotalCommits: 500,
				PullRequests: "12",
				Issues:       "34",
				Contributors: 150,
			},
			LanguagesBreakDown: map[string]string{
				"Go": "99.2%",
			},
		},
	}

	th := theme.Catalog[0]
	tbl := InitialTable(repos, 10, th)
	lst := InitialFileList(repos[0].RootInfor)
	vp := viewport.New(60, 20)

	m := Model{
		state:    stateDashboard,
		theme:    th,
		table:    tbl,
		list:     lst,
		repoList: repos,
		viewport: vp,
		active:   tableActive,
		width:    80,
		height:   24,
	}

	// Test Wide layout
	m.recalculateLayout(140, 40)
	if m.width != 140 || m.height != 40 {
		t.Errorf("expected dimensions 140x40, got %dx%d", m.width, m.height)
	}
	wideView := m.View()
	if len(wideView) == 0 {
		t.Errorf("wideView should not be empty")
	}

	// Test Compact layout
	m.recalculateLayout(80, 24)
	if m.width != 80 || m.height != 24 {
		t.Errorf("expected dimensions 80x24, got %dx%d", m.width, m.height)
	}
	compactView := m.View()
	if len(compactView) == 0 {
		t.Errorf("compactView should not be empty")
	}

	// Test Compact Readme view
	m.active = readmeActive
	readmeView := m.View()
	if len(readmeView) == 0 {
		t.Errorf("readmeView should not be empty")
	}

	// Test Theme Modal interaction
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	tm := updated.(Model)
	if !tm.showThemeModal {
		t.Errorf("expected theme modal to be open")
	}

	modalView := tm.View()
	if !strings.Contains(modalView, "SELECT THEME") {
		t.Errorf("expected modal view to contain title, got %q", modalView)
	}

	// Test navigation in modal
	downUpdated, _ := tm.Update(tea.KeyMsg{Type: tea.KeyDown})
	dm := downUpdated.(Model)
	if dm.themeIndex != 1 {
		t.Errorf("expected themeIndex 1 after down key, got %d", dm.themeIndex)
	}

	// Test enter to select
	enterUpdated, _ := dm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	em := enterUpdated.(Model)
	if em.showThemeModal {
		t.Errorf("expected modal to close after Enter")
	}
	if em.theme.ID != theme.Catalog[1].ID {
		t.Errorf("expected theme to change to %s, got %s", theme.Catalog[1].ID, em.theme.ID)
	}

	// Test user pill in status bar
	m.cfg = &flags.CmdConfig{User: "torvalds"}
	m.active = tableActive
	bar := m.renderStatusBar(100)
	if !strings.Contains(bar, "@torvalds") {
		t.Errorf("expected status bar to contain '@torvalds', got %q", bar)
	}
}

func TestView_NeverExceedsTerminalBounds(t *testing.T) {
	repos := []Repo{
		{
			Owner:       "elementalsouls",
			Name:        "Claude-BugHunter",
			Stars:       "16,634",
			Forks:       "1,881",
			Language:    "Python",
			Description: "Built by Sachin Sharma",
			ReadMe:      "# claude-bughunter\n\nBuilt by Sachin Sharma",
			RootInfor: []EntryInfor{
				{Type: "dir", Name: ".claude-plugin"},
				{Type: "file", Name: "README.md"},
			},
			ExtraInfor: ExtraInfor{
				TotalCommits: 356,
				PullRequests: "15",
				Issues:       "5",
				Contributors: 19,
			},
			LanguagesBreakDown: map[string]string{
				"Python": "80%",
				"Shell":  "20%",
			},
		},
	}

	widths := []int{60, 65, 70, 80, 90, 100, 110, 120, 140, 160}
	heights := []int{14, 16, 18, 20, 22, 24, 26, 28, 30, 35, 40, 50}
	modes := []activeComponent{tableActive, listActive, readmeActive}

	for _, th := range theme.Catalog {
		for _, w := range widths {
			for _, h := range heights {
				for _, act := range modes {
					tbl := InitialTable(repos, 10, th)
					lst := InitialFileList(repos[0].RootInfor)
					vp := viewport.New(60, 20)

					m := Model{
						state:    stateDashboard,
						theme:    th,
						table:    tbl,
						list:     lst,
						repoList: repos,
						viewport: vp,
						active:   act,
						width:    w,
						height:   h,
					}
					m.recalculateLayout(w, h)
					v := m.View()

					actualH := lipgloss.Height(v)
					actualW := lipgloss.Width(v)

					if actualH > h {
						t.Errorf("Theme=%s Size=%dx%d Active=%d: view height %d exceeds terminal height %d (overflow by %d lines)",
							th.ID, w, h, act, actualH, h, actualH-h)
					}
					if actualW > w {
						t.Errorf("Theme=%s Size=%dx%d Active=%d: view width %d exceeds terminal width %d (overflow by %d cols)",
							th.ID, w, h, act, actualW, w, actualW-w)
					}

					// Verify headers are never cut off in tableActive or wide mode
					if act == tableActive || w >= 110 {
						if !strings.Contains(v, "Owner") {
							t.Errorf("Theme=%s Size=%dx%d: expected view to contain 'Owner' table header", th.ID, w, h)
						}
						if !strings.Contains(v, "Repo name") {
							t.Errorf("Theme=%s Size=%dx%d: expected view to contain 'Repo name' table header", th.ID, w, h)
						}
						// Verify bottom border is not chopped off
						if !strings.Contains(v, "╰") {
							t.Errorf("Theme=%s Size=%dx%d: expected view to contain table bottom border corner '╰'", th.ID, w, h)
						}
					}
				}
			}
		}
	}
}

func TestThemeModal_NeverExceedsTerminalHeight(t *testing.T) {
	repos := []Repo{{Owner: "test", Name: "repo", Stars: "1", Forks: "1", Language: "Go"}}
	for _, h := range []int{14, 15, 16, 17, 20, 24, 30} {
		for _, w := range []int{60, 80, 120} {
			th := theme.Catalog[0]
			tbl := InitialTable(repos, 10, th)
			lst := InitialFileList(nil)
			vp := viewport.New(60, 20)
			m := Model{
				state:          stateDashboard,
				theme:          th,
				table:          tbl,
				list:           lst,
				repoList:       repos,
				viewport:       vp,
				showThemeModal: true,
				width:          w,
				height:         h,
			}
			v := m.View()
			actualH := lipgloss.Height(v)
			if actualH > h {
				t.Errorf("ThemeModal Size=%dx%d: view height %d exceeds terminal height %d", w, h, actualH, h)
			}
		}
	}
}

func TestLanguages_SortNumerically(t *testing.T) {
	repos := []Repo{
		{
			Owner: "test",
			Name:  "repo",
			LanguagesBreakDown: map[string]string{
				"Python": "9.5%",
				"Go":     "80.0%",
				"Shell":  "10.5%",
			},
		},
	}
	th := theme.Catalog[0]
	m := Model{
		repoList: repos,
		theme:    th,
	}
	rendered := m.renderLanguagesBreakDown(40, 10, th)
	goIdx := strings.Index(rendered, "Go")
	shellIdx := strings.Index(rendered, "Shell")
	pyIdx := strings.Index(rendered, "Python")

	if goIdx == -1 || shellIdx == -1 || pyIdx == -1 {
		t.Fatalf("expected all languages in breakdown, got %q", rendered)
	}
	if !(goIdx < shellIdx && shellIdx < pyIdx) {
		t.Errorf("expected sorted order Go (80%%) -> Shell (10.5%%) -> Python (9.5%%), got %q", rendered)
	}
}

func TestView_TooSmallWindow(t *testing.T) {
	repos := []Repo{
		{
			Owner:    "test",
			Name:     "repo",
			Stars:    "100",
			Forks:    "10",
			Language: "Go",
		},
	}
	th := theme.Catalog[0]
	tbl := InitialTable(repos, 10, th)
	lst := InitialFileList(nil)
	vp := viewport.New(20, 10)

	smallSizes := [][2]int{
		{50, 10},
		{40, 20},
		{59, 13},
		{59, 24},
		{80, 13},
		{30, 8},
		{15, 4},
	}

	for _, size := range smallSizes {
		w, h := size[0], size[1]
		m := Model{
			state:    stateDashboard,
			theme:    th,
			table:    tbl,
			list:     lst,
			repoList: repos,
			viewport: vp,
			width:    w,
			height:   h,
		}

		v := m.View()
		if !strings.Contains(v, "Window Too Small") && !strings.Contains(v, "Window too small") && !strings.Contains(v, "Too small") {
			t.Errorf("Size=%dx%d: expected view to contain 'Window Too Small' notice, got %q", w, h, v)
		}

		actualH := lipgloss.Height(v)
		if actualH > h {
			t.Errorf("Size=%dx%d: view height %d exceeds terminal height %d", w, h, actualH, h)
		}
		actualW := lipgloss.Width(v)
		if actualW > w {
			t.Errorf("Size=%dx%d: view width %d exceeds terminal width %d", w, h, actualW, w)
		}
	}
}
