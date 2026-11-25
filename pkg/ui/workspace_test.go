package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newTestWorkspaceModel() Model {
	svc := &WorkspaceService{
		FetchMetadata: func(ctx context.Context, owner, repo string) (ghclient.RepoMetadata, error) {
			return ghclient.RepoMetadata{
				Owner:         owner,
				Name:          repo,
				FullName:      owner + "/" + repo,
				Description:   "A delightful Go TUI framework",
				DefaultBranch: "main",
				License:       "MIT",
				Language:      "Go",
				HTMLURL:       "https://github.com/" + owner + "/" + repo,
				Stars:         28000,
				Forks:         850,
				Watchers:      210,
				Topics:        []string{"tui", "go", "bubbletea"},
			}, nil
		},
		FetchRefs: func(ctx context.Context, owner, repo string) ([]ghclient.RefInfo, error) {
			return []ghclient.RefInfo{
				{Name: "main", SHA: "111", Kind: "branch"},
				{Name: "dev", SHA: "222", Kind: "branch"},
				{Name: "v1.3.10", SHA: "333", Kind: "tag"},
			}, nil
		},
		FetchTree: func(ctx context.Context, owner, repo, ref string) ([]ghclient.TreeEntry, bool, error) {
			if ref == "dev" {
				return []ghclient.TreeEntry{
					{Path: "dev_only.go", Name: "dev_only.go", Type: "blob"},
				}, false, nil
			}
			return []ghclient.TreeEntry{
				{Path: "README.md", Name: "README.md", Type: "blob"},
				{Path: "pkg", Name: "pkg", Type: "tree"},
				{Path: "pkg/main.go", Name: "main.go", Type: "blob"},
				{Path: "go.mod", Name: "go.mod", Type: "blob"},
			}, false, nil
		},
		FetchFile: func(ctx context.Context, owner, repo, filePath, ref string) (ghclient.FileContent, error) {
			return ghclient.FileContent{
				Path:    filePath,
				Ref:     ref,
				Content: "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
				Size:    45,
			}, nil
		},
		FetchREADME: func(ctx context.Context, owner, repo, ref string) (string, error) {
			return "# " + repo + "\n\nRef: " + ref + "\n\n```go\nfmt.Println(\"ok\")\n```", nil
		},
	}

	m := newTestHomeModel(nil)
	m.launchOpts.Workspace = svc
	m.repoList = testSampleRepos("charmbracelet", "bubbletea")
	m.filteredRepos = m.repoList
	m.table = InitialTableWithMode(m.repoList, 10, m.theme, false)
	m.list = InitialFileList(nil)
	m.viewport = viewport.New(70, 20)
	m.state = stateDashboard
	m.recalculateLayout(120, 34)
	return m
}

func TestWorkspace_OpenTreeFileSwitchBranchAndBack(t *testing.T) {
	m := newTestWorkspaceModel()

	// Open workspace via Enter from Dashboard
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.state != stateWorkspace || m.workspace.owner != "charmbracelet" || m.workspace.repo != "bubbletea" {
		t.Fatalf("expected stateWorkspace for charmbracelet/bubbletea, got state=%v owner=%q repo=%q", m.state, m.workspace.owner, m.workspace.repo)
	}
	for _, msg := range execBatchMsgs(cmd) {
		updated, post := m.Update(msg)
		m = updated.(Model)
		for _, pmsg := range execBatchMsgs(post) {
			updated, _ = m.Update(pmsg)
			m = updated.(Model)
		}
	}

	if m.workspace.activeRef != "main" {
		t.Fatalf("expected activeRef=main from metadata, got %q", m.workspace.activeRef)
	}
	// Root directory sorted: pkg (tree), go.mod, README.md
	if len(m.workspace.visibleNodes) != 3 || m.workspace.visibleNodes[0].entry.Path != "pkg" {
		t.Fatalf("unexpected visible root nodes: %+v", m.workspace.visibleNodes)
	}

	// Expand pkg folder -> child pkg/main.go appears
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if len(m.workspace.visibleNodes) != 4 || m.workspace.visibleNodes[1].entry.Path != "pkg/main.go" {
		t.Fatalf("expected expanded pkg/main.go, got %+v", m.workspace.visibleNodes)
	}

	// Move down to pkg/main.go and open file
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, fileCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(fileCmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}
	if m.workspace.viewerPath != "pkg/main.go" || m.workspace.codeFocus != wsFocusViewer {
		t.Fatalf("expected viewer focused on pkg/main.go, got path=%q focus=%v", m.workspace.viewerPath, m.workspace.codeFocus)
	}
	if !strings.Contains(m.workspace.viewer.View(), "package") {
		t.Fatalf("expected highlighted code in viewer")
	}

	// Press 'b' to open Branch Switcher, filter "dev", ensure 'q' doesn't quit while typing, select dev
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)
	if !m.workspace.showBranchModal {
		t.Fatalf("expected branch modal open")
	}
	for _, r := range "dev" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, switchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	for _, msg := range execBatchMsgs(switchCmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}
	if m.workspace.activeRef != "dev" || len(m.workspace.visibleNodes) != 1 || m.workspace.visibleNodes[0].entry.Path != "dev_only.go" {
		t.Fatalf("expected switched to dev branch tree, got ref=%q nodes=%+v", m.workspace.activeRef, m.workspace.visibleNodes)
	}

	// Stale refSeq response from prior branch must be ignored
	updated, _ = m.Update(wsTreeLoadedMsg{
		seq:     m.workspace.seq,
		refSeq:  m.workspace.refSeq - 1,
		entries: []ghclient.TreeEntry{{Path: "stale.go", Name: "stale.go", Type: "blob"}},
	})
	m = updated.(Model)
	if len(m.workspace.visibleNodes) != 1 || m.workspace.visibleNodes[0].entry.Path != "dev_only.go" {
		t.Fatalf("stale refSeq tree overwrote active branch: %+v", m.workspace.visibleNodes)
	}

	// Truncated tree fallback dirChildren merges cleanly even when allTree is non-empty
	m.workspace.treeTruncated = true
	m.workspace.allTree = []ghclient.TreeEntry{{Path: "bigdir", Name: "bigdir", Type: "tree"}}
	m.workspace.expanded = map[string]bool{"bigdir": true}
	updated, _ = m.Update(wsDirLoadedMsg{
		seq:     m.workspace.seq,
		refSeq:  m.workspace.refSeq,
		dirPath: "bigdir",
		entries: []ghclient.TreeEntry{{Path: "bigdir/deep.go", Name: "deep.go", Type: "blob"}},
	})
	m = updated.(Model)
	if len(m.workspace.visibleNodes) != 2 || m.workspace.visibleNodes[1].entry.Path != "bigdir/deep.go" {
		t.Fatalf("expected dirChildren fallback to render under partial allTree, got %+v", m.workspace.visibleNodes)
	}

	// Switch tabs with 2, 3, 4, 1
	for _, k := range []rune{'2', '3', '4', '1'} {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = updated.(Model)
	}
	if m.workspace.tab != wsTabCode {
		t.Fatalf("expected tab=wsTabCode, got %v", m.workspace.tab)
	}

	// Esc returns cleanly to prevState (stateDashboard)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.state != stateDashboard {
		t.Fatalf("expected Esc to return to stateDashboard, got %v", m.state)
	}
}

func TestWorkspaceView_NeverExceedsBoundsAllThemes(t *testing.T) {
	sizes := []struct{ w, h int }{
		{60, 14},
		{80, 24},
		{100, 30},
		{140, 40},
	}
	for _, th := range theme.Catalog {
		for _, sz := range sizes {
			m := newTestWorkspaceModel()
			m.theme = th
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
			m = updated.(Model)
			for _, msg := range execBatchMsgs(cmd) {
				updated, _ = m.Update(msg)
				m = updated.(Model)
			}
			m.width = sz.w
			m.height = sz.h
			m.recalculateWorkspaceLayout(sz.w, sz.h)
			out := m.View()
			lines := strings.Split(out, "\n")
			if len(lines) > sz.h {
				t.Fatalf("theme=%s size=%dx%d workspace height=%d > %d", th.ID, sz.w, sz.h, len(lines), sz.h)
			}
			for i, ln := range lines {
				if lw := lipgloss.Width(ln); lw > sz.w {
					t.Fatalf("theme=%s size=%dx%d line %d width=%d > %d", th.ID, sz.w, sz.h, i+1, lw, sz.w)
				}
			}
		}
	}
}
