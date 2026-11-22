package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
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

	tbl := InitialTable(repos, 10)
	lst := InitialFileList(repos[0].RootInfor)
	vp := viewport.New(60, 20)

	m := Model{
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
}
