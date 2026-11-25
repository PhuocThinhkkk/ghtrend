package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"github.com/PhuocThinhkkk/ghtrend/pkg/theme"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderUnifiedDiff(t *testing.T) {
	patch := "@@ -10,3 +10,4 @@ func hello() {\n \tctx := context.Background()\n-\tprintln(\"old\")\n+\tprintln(\"new\")\n+\tprintln(\"extra\")\n"
	out := stripTestANSI(RenderUnifiedDiff(patch, theme.Catalog[0], 80))
	if !strings.Contains(out, "@@ -10,3 +10,4 @@") {
		t.Fatalf("expected hunk header in diff output, got:\n%s", out)
	}
	if !strings.Contains(out, "  11     │ -") {
		t.Fatalf("expected old line number 11 in deletion, got:\n%s", out)
	}
	if !strings.Contains(out, "      11 │ +") {
		t.Fatalf("expected new line number 11 in addition, got:\n%s", out)
	}
	if !strings.Contains(RenderUnifiedDiff("", theme.Catalog[0], 80), "No textual diff available") {
		t.Fatalf("expected empty patch message")
	}
}

func TestWorkspaceCommitsDiffAndQuickActions(t *testing.T) {
	starredState := false
	clonedTo := ""
	svc := &WorkspaceService{
		FetchCommits: func(ctx context.Context, owner, repo, ref string, page int) ([]ghclient.CommitSummary, bool, error) {
			return []ghclient.CommitSummary{
				{
					SHA:         "abc1234567890",
					ShortSHA:    "abc1234",
					Message:     "feat: add diff viewer",
					Author:      "Alice",
					AuthorLogin: "alice",
					Date:        time.Now().Add(-2 * time.Hour),
				},
			}, true, nil
		},
		FetchCommitDiff: func(ctx context.Context, owner, repo, sha string) (ghclient.CommitDiff, error) {
			return ghclient.CommitDiff{
				SHA:          sha,
				ShortSHA:     "abc1234",
				Message:      "feat: add diff viewer",
				Additions:    2,
				Deletions:    1,
				ChangedFiles: 2,
				Files: []ghclient.CommitFileDiff{
					{Filename: "pkg/ui/diff.go", Status: "added", Additions: 2, Deletions: 0, Patch: "@@ -0,0 +1,2 @@\n+package ui\n+// ok\n"},
					{Filename: "README.md", Status: "modified", Additions: 0, Deletions: 1, Patch: "@@ -1,2 +1,1 @@\n # Title\n-old line\n"},
				},
			}, nil
		},
		IsStarred: func(ctx context.Context, owner, repo string) (bool, error) {
			return starredState, nil
		},
		SetStarred: func(ctx context.Context, owner, repo string, star bool) error {
			starredState = star
			return nil
		},
		CloneRepo: func(ctx context.Context, owner, repo, parentDir string) (string, error) {
			clonedTo = owner + "/" + repo
			return "./" + repo, nil
		},
	}

	m := newTestPRIssueWorkspaceModel(svc)

	// Switch to 4:Commits
	nextModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = nextModel.(Model)
	if m.workspace.tab != wsTabCommits {
		t.Fatalf("expected wsTabCommits, got %v", m.workspace.tab)
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)

	view := stripTestANSI(m.View())
	if !strings.Contains(view, "abc1234") || !strings.Contains(view, "feat: add diff viewer") {
		t.Fatalf("expected commit row in view, got:\n%s", view)
	}

	// Press Enter to open Commit Diff
	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nextModel.(Model)
	if !m.workspace.commitDiffOpen {
		t.Fatalf("expected commitDiffOpen true")
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)

	diffView := stripTestANSI(m.View())
	if !strings.Contains(diffView, "pkg/ui/diff.go") || !strings.Contains(diffView, "+package ui") {
		t.Fatalf("expected diff files and patch in view, got:\n%s", diffView)
	}

	// Move to second file and verify diff updates
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nextModel.(Model)
	diffView = stripTestANSI(m.View())
	if !strings.Contains(diffView, "-old line") {
		t.Fatalf("expected second file diff in view, got:\n%s", diffView)
	}

	// Esc closes diff back to commit list, not workspace
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nextModel.(Model)
	if m.workspace.commitDiffOpen || m.state != stateWorkspace {
		t.Fatalf("expected Esc from commit diff to stay in workspace commit list")
	}

	// Quick actions: 's' star toggle, 'C' clone repo
	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = nextModel.(Model)
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)
	if !starredState || !m.workspace.starred {
		t.Fatalf("expected repo to be starred after 's'")
	}

	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	m = nextModel.(Model)
	for _, msg := range execBatchMsgs(cmd) {
		nextModel, _ = m.Update(msg)
		m = nextModel.(Model)
	}
	if clonedTo != "octocat/hello-world" {
		t.Fatalf("expected clone for octocat/hello-world, got %q", clonedTo)
	}
}

func TestGlobalSearchModalWorkflow(t *testing.T) {
	m := newTestHomeModel(nil)
	m.launchOpts.Search = &SearchService{
		SearchRepos: func(ctx context.Context, query string, page int) ([]ghclient.SearchRepoResult, bool, error) {
			return []ghclient.SearchRepoResult{
				{FullName: "charmbracelet/bubbletea", Owner: "charmbracelet", Name: "bubbletea", Stars: 29000, Language: "Go"},
			}, false, nil
		},
		SearchUsers: func(ctx context.Context, query string, page int) ([]ghclient.SearchUserResult, bool, error) {
			return []ghclient.SearchUserResult{
				{Login: "torvalds", Name: "Linus Torvalds", Type: "User"},
			}, false, nil
		},
	}
	m.launchOpts.Workspace = &WorkspaceService{
		FetchMetadata: func(ctx context.Context, owner, repo string) (ghclient.RepoMetadata, error) {
			return ghclient.RepoMetadata{Owner: owner, Name: repo, DefaultBranch: "main"}, nil
		},
		FetchRefs: func(ctx context.Context, owner, repo string) ([]ghclient.RefInfo, error) {
			return nil, nil
		},
		FetchTree: func(ctx context.Context, owner, repo, ref string) ([]ghclient.TreeEntry, bool, error) {
			return nil, false, nil
		},
		FetchREADME: func(ctx context.Context, owner, repo, ref string) (string, error) {
			return "# " + repo, nil
		},
		IsStarred: func(ctx context.Context, owner, repo string) (bool, error) {
			return false, nil
		},
	}

	// Open global search with '/'
	nextModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = nextModel.(Model)
	if !m.searchModal.open {
		t.Fatalf("expected searchModal open after '/'")
	}

	// Type 'bubble'
	for _, r := range "bubble" {
		nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = nextModel.(Model)
	}

	// Simulate debounce firing
	nextModel, cmd := m.Update(globalSearchDebounceMsg{seq: m.searchModal.seq, mode: searchModeRepos, query: "bubble"})
	m = nextModel.(Model)
	if cmd == nil {
		t.Fatalf("expected search cmd from debounce msg")
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)
	if len(m.searchModal.repos) != 1 || m.searchModal.repos[0].FullName != "charmbracelet/bubbletea" {
		t.Fatalf("expected repo search result, got %+v", m.searchModal.repos)
	}

	// Toggle mode to Users via Tab
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = nextModel.(Model)
	if m.searchModal.mode != searchModeUsers {
		t.Fatalf("expected searchModeUsers after Tab, got %q", m.searchModal.mode)
	}
	nextModel, cmd = m.Update(globalSearchDebounceMsg{seq: m.searchModal.seq, mode: searchModeUsers, query: "bubble"})
	m = nextModel.(Model)
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)
	if len(m.searchModal.users) != 1 || m.searchModal.users[0].Login != "torvalds" {
		t.Fatalf("expected user search result, got %+v", m.searchModal.users)
	}

	// Switch back to Repos and press Enter to open workspace
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = nextModel.(Model)
	nextModel, cmd = m.Update(globalSearchDebounceMsg{seq: m.searchModal.seq, mode: searchModeRepos, query: "bubble"})
	m = nextModel.(Model)
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)

	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nextModel.(Model)
	if m.searchModal.open {
		t.Fatalf("expected searchModal closed on Enter")
	}
	if m.state != stateWorkspace || m.workspace.owner != "charmbracelet" || m.workspace.repo != "bubbletea" {
		t.Fatalf("expected stateWorkspace for charmbracelet/bubbletea, got state=%v %s/%s", m.state, m.workspace.owner, m.workspace.repo)
	}
}
