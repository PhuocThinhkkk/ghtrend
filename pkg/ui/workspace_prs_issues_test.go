package ui

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	tea "github.com/charmbracelet/bubbletea"
)

var testANSIRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripTestANSI(s string) string {
	return testANSIRegex.ReplaceAllString(s, "")
}

func newTestPRIssueWorkspaceModel(svc *WorkspaceService) Model {
	if svc.FetchMetadata == nil {
		svc.FetchMetadata = func(ctx context.Context, owner, repo string) (ghclient.RepoMetadata, error) {
			return ghclient.RepoMetadata{Owner: owner, Name: repo, DefaultBranch: "main"}, nil
		}
	}
	if svc.FetchRefs == nil {
		svc.FetchRefs = func(ctx context.Context, owner, repo string) ([]ghclient.RefInfo, error) {
			return []ghclient.RefInfo{{Name: "main", Kind: "branch"}}, nil
		}
	}
	if svc.FetchTree == nil {
		svc.FetchTree = func(ctx context.Context, owner, repo, ref string) ([]ghclient.TreeEntry, bool, error) {
			return nil, false, nil
		}
	}
	if svc.FetchDirContents == nil {
		svc.FetchDirContents = func(ctx context.Context, owner, repo, dirPath, ref string) ([]ghclient.TreeEntry, error) {
			return nil, nil
		}
	}
	if svc.FetchFile == nil {
		svc.FetchFile = func(ctx context.Context, owner, repo, filePath, ref string) (ghclient.FileContent, error) {
			return ghclient.FileContent{}, nil
		}
	}
	if svc.FetchREADME == nil {
		svc.FetchREADME = func(ctx context.Context, owner, repo, ref string) (string, error) {
			return "# README", nil
		}
	}

	m := newTestHomeModel(nil)
	m.launchOpts.Workspace = svc
	m.width = 120
	m.height = 36
	m.openWorkspace("octocat", "hello-world", "Home", stateHome)
	return m
}

func TestWorkspacePRsTabAndCommentModal(t *testing.T) {
	prFetchCalls := 0
	var postedBody string
	svc := &WorkspaceService{
		FetchPRs: func(ctx context.Context, owner, repo, state string, page int) ([]ghclient.PullRequest, bool, error) {
			prFetchCalls++
			return []ghclient.PullRequest{
				{
					Number:         42,
					Title:          "Add TUI workspace",
					Body:           "Implements multi-tab workspace.",
					State:          "open",
					Author:         "alice",
					HeadRef:        "feat/ws",
					BaseRef:        "main",
					ReviewDecision: "APPROVED",
					CheckState:     "SUCCESS",
					CommentCount:   1,
				},
			}, true, nil
		},
		FetchPRTimeline: func(ctx context.Context, owner, repo string, number int) ([]ghclient.TimelineComment, error) {
			return []ghclient.TimelineComment{
				{ID: "1", Author: "bob", Body: "Looks great!", Kind: "review", State: "APPROVED", CreatedAt: time.Now()},
			}, nil
		},
		CreateComment: func(ctx context.Context, owner, repo string, number int, body string) (ghclient.TimelineComment, error) {
			postedBody = body
			return ghclient.TimelineComment{ID: "2", Author: "me", Body: body, Kind: "issue_comment", CreatedAt: time.Now()}, nil
		},
	}

	m := newTestPRIssueWorkspaceModel(svc)

	// Switch to 2:PRs triggers lazy load.
	nextModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = nextModel.(Model)
	if m.workspace.tab != wsTabPRs {
		t.Fatalf("expected wsTabPRs, got %v", m.workspace.tab)
	}
	if cmd == nil {
		t.Fatalf("expected PR fetch cmd")
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)
	if prFetchCalls != 1 {
		t.Fatalf("expected 1 PR fetch call, got %d", prFetchCalls)
	}

	view := m.View()
	if !strings.Contains(view, "#42 Add TUI workspace") || !strings.Contains(view, "feat/ws") {
		t.Fatalf("expected PR row in view, got:\n%s", view)
	}

	// Cycle filter with 'f': open -> closed.
	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = nextModel.(Model)
	if m.workspace.prFilter != "closed" {
		t.Fatalf("expected prFilter closed, got %q", m.workspace.prFilter)
	}
	if cmd != nil {
		nextModel, _ = m.Update(cmd())
		m = nextModel.(Model)
	}

	// Open PR detail with Enter.
	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nextModel.(Model)
	if m.workspace.prDetail == nil || m.workspace.prDetail.Number != 42 {
		t.Fatalf("expected PR #42 detail open")
	}
	if cmd != nil {
		nextModel, _ = m.Update(cmd())
		m = nextModel.(Model)
	}
	detailView := stripTestANSI(m.View())
	if !strings.Contains(detailView, "Looks great!") {
		t.Fatalf("expected timeline comment in PR detail, got:\n%s", detailView)
	}

	// Open comment modal with 'c' and type 'qHt?hello'.
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = nextModel.(Model)
	if !m.workspace.showCommentModal {
		t.Fatalf("expected comment modal open")
	}

	// Empty submit should fail locally.
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = nextModel.(Model)
	if m.workspace.commentErr == nil {
		t.Fatalf("expected empty comment error")
	}

	// Typing global hotkeys 'q', 'H', 't', '?' inside textarea must NOT trigger app actions.
	for _, r := range "qHt? LGTM" {
		nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = nextModel.(Model)
	}
	if m.state != stateWorkspace || !m.workspace.showCommentModal {
		t.Fatalf("expected to remain in workspace comment modal while typing")
	}

	// Submit with Ctrl+S.
	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = nextModel.(Model)
	if cmd == nil {
		t.Fatalf("expected CreateComment cmd")
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)

	if postedBody != "qHt? LGTM" {
		t.Fatalf("expected postedBody %q, got %q", "qHt? LGTM", postedBody)
	}
	if m.workspace.showCommentModal {
		t.Fatalf("expected comment modal to close after submit")
	}
	if m.workspace.prDetail.CommentCount != 2 {
		t.Fatalf("expected PR comment count 2, got %d", m.workspace.prDetail.CommentCount)
	}

	// Esc closes PR detail back to PR list, NOT workspace.
	nextModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nextModel.(Model)
	if m.workspace.prDetail != nil || m.state != stateWorkspace {
		t.Fatalf("expected Esc from detail to return to PR list within workspace")
	}
}

func TestWorkspaceIssuesTabAndStateToggle(t *testing.T) {
	var updatedState string
	svc := &WorkspaceService{
		FetchIssues: func(ctx context.Context, owner, repo, state string, page int) ([]ghclient.Issue, bool, error) {
			return []ghclient.Issue{
				{
					Number:       7,
					Title:        "Fix layout clipping",
					Body:         "Details here",
					State:        "open",
					Author:       "carol",
					Assignees:    []string{"dave"},
					Labels:       []ghclient.Label{{Name: "bug", Color: "d73a4a"}},
					CommentCount: 0,
				},
			}, false, nil
		},
		FetchIssueTimeline: func(ctx context.Context, owner, repo string, number int) ([]ghclient.TimelineComment, error) {
			return nil, nil
		},
		UpdateIssueState: func(ctx context.Context, owner, repo string, number int, state string) (ghclient.Issue, error) {
			updatedState = state
			return ghclient.Issue{Number: number, Title: "Fix layout clipping", State: state}, nil
		},
	}

	m := newTestPRIssueWorkspaceModel(svc)

	// Switch to 3:Issues.
	nextModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = nextModel.(Model)
	if m.workspace.tab != wsTabIssues {
		t.Fatalf("expected wsTabIssues, got %v", m.workspace.tab)
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)

	view := m.View()
	if !strings.Contains(view, "#7 Fix layout clipping") {
		t.Fatalf("expected issue row in view, got:\n%s", view)
	}

	// Toggle issue state with 'X'.
	nextModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	m = nextModel.(Model)
	if cmd == nil {
		t.Fatalf("expected UpdateIssueState cmd")
	}
	nextModel, _ = m.Update(cmd())
	m = nextModel.(Model)

	if updatedState != "closed" {
		t.Fatalf("expected updatedState closed, got %q", updatedState)
	}
	if m.workspace.issues[0].State != "closed" {
		t.Fatalf("expected issue state closed in slice, got %q", m.workspace.issues[0].State)
	}

	// Stale seq message should be ignored.
	nextModel, _ = m.Update(wsIssuesLoadedMsg{seq: m.workspace.seq + 99, issues: nil})
	m = nextModel.(Model)
	if len(m.workspace.issues) != 1 {
		t.Fatalf("expected stale seq message to be ignored")
	}
}
