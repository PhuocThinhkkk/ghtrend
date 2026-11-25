package ghclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupIssuesPRsTestClient(t *testing.T, token string, handler http.Handler) {
	t.Helper()

	srv := httptest.NewServer(handler)
	defaultClientMu.Lock()
	origClient := DefaultClient
	DefaultClient = &Client{
		APIBaseURL: srv.URL,
		GraphQLURL: srv.URL + "/graphql",
		WebBaseURL: srv.URL,
		HTTPClient: srv.Client(),
		TokenFunc:  func() string { return token },
		Cache:      nil,
		accepts:    newAcceptSet(),
		creds:      newAcceptSet(),
	}
	defaultClientMu.Unlock()

	t.Cleanup(func() {
		defaultClientMu.Lock()
		DefaultClient = origClient
		defaultClientMu.Unlock()
		srv.Close()
	})
}

func TestFetchPullRequests(t *testing.T) {
	t.Run("graphql success on page 1", func(t *testing.T) {
		var sawGraphQL, sawREST bool
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/graphql" {
				sawGraphQL = true
				var payload struct {
					Query     string         `json:"query"`
					Variables map[string]any `json:"variables"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode graphql payload: %v", err)
				}
				if payload.Variables["owner"] != "octocat" || payload.Variables["repo"] != "hello-world" {
					t.Fatalf("unexpected variables: %+v", payload.Variables)
				}
				_, _ = w.Write([]byte(`{
					"data": {
						"repository": {
							"pullRequests": {
								"nodes": [
									{
										"number": 42,
										"title": "Add TUI PR detail view",
										"body": "Implements PR detail",
										"state": "OPEN",
										"isDraft": false,
										"merged": false,
										"url": "https://github.com/octocat/hello-world/pull/42",
										"headRefName": "feat/pr-detail",
										"baseRefName": "main",
										"reviewDecision": "APPROVED",
										"additions": 120,
										"deletions": 15,
										"changedFiles": 4,
										"createdAt": "2026-11-20T10:00:00Z",
										"updatedAt": "2026-11-21T12:00:00Z",
										"author": {"login": "octocat"},
										"comments": {"totalCount": 5},
										"labels": {"nodes": [{"name": "enhancement", "color": "a2eeef", "description": "New feature"}]},
										"commits": {"nodes": [{"commit": {"statusCheckRollup": {"state": "SUCCESS"}}}]}
									},
									{
										"number": 41,
										"title": "Ghost author merged PR",
										"body": "",
										"state": "MERGED",
										"isDraft": true,
										"merged": true,
										"url": "https://github.com/octocat/hello-world/pull/41",
										"headRefName": "fix/ghost",
										"baseRefName": "main",
										"reviewDecision": "",
										"additions": 3,
										"deletions": 1,
										"changedFiles": 1,
										"createdAt": "2026-11-19T10:00:00Z",
										"updatedAt": "2026-11-19T11:00:00Z",
										"author": null,
										"comments": {"totalCount": 0},
										"labels": {"nodes": []},
										"commits": {"nodes": []}
									}
								],
								"pageInfo": {"hasNextPage": true}
							}
						}
					}
				}`))
				return
			}
			sawREST = true
			w.WriteHeader(http.StatusNotFound)
		}))

		prs, hasMore, err := FetchPullRequests(context.Background(), "octocat", "hello-world", "all", 1)
		if err != nil {
			t.Fatalf("FetchPullRequests() error = %v", err)
		}
		if !sawGraphQL || sawREST {
			t.Fatalf("expected GraphQL only, sawGraphQL=%v sawREST=%v", sawGraphQL, sawREST)
		}
		if !hasMore {
			t.Fatalf("expected hasMore=true")
		}
		if len(prs) != 2 {
			t.Fatalf("expected 2 PRs, got %d", len(prs))
		}
		if prs[0].Number != 42 || prs[0].State != "open" || prs[0].Author != "octocat" ||
			prs[0].HeadRef != "feat/pr-detail" || prs[0].BaseRef != "main" ||
			prs[0].ReviewDecision != "APPROVED" || prs[0].CheckState != "SUCCESS" ||
			prs[0].Additions != 120 || prs[0].Deletions != 15 || prs[0].ChangedFiles != 4 ||
			prs[0].CommentCount != 5 || len(prs[0].Labels) != 1 || prs[0].Labels[0].Name != "enhancement" {
			t.Fatalf("unexpected first PR: %+v", prs[0])
		}
		if prs[1].Number != 41 || prs[1].State != "closed" || !prs[1].Merged || !prs[1].Draft || prs[1].Author != "" {
			t.Fatalf("unexpected second PR: %+v", prs[1])
		}
	})

	t.Run("graphql error falls back to REST", func(t *testing.T) {
		var sawGraphQL, sawREST bool
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/graphql" {
				sawGraphQL = true
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message": "internal error"}`))
				return
			}
			if r.URL.Path == "/repos/octocat/hello-world/pulls" {
				sawREST = true
				if got := r.URL.Query().Get("state"); got != "open" {
					t.Fatalf("expected state=open, got %q", got)
				}
				if got := r.URL.Query().Get("per_page"); got != "30" {
					t.Fatalf("expected per_page=30, got %q", got)
				}
				if got := r.URL.Query().Get("page"); got != "1" {
					t.Fatalf("expected page=1, got %q", got)
				}
				w.Header().Set("Link", `<https://api.github.com/repos/octocat/hello-world/pulls?page=2>; rel="next"`)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[
					{
						"number": 7,
						"title": "REST fallback PR",
						"body": "Body text",
						"state": "open",
						"draft": false,
						"html_url": "https://github.com/octocat/hello-world/pull/7",
						"merged_at": null,
						"comments": 2,
						"created_at": "2026-11-20T09:00:00Z",
						"updated_at": "2026-11-20T10:00:00Z",
						"user": {"login": "alice"},
						"labels": [{"name": "bug", "color": "d73a4a", "description": "Bug"}],
						"head": {"ref": "fix/rest"},
						"base": {"ref": "main"}
					}
				]`))
				return
			}
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}))

		prs, hasMore, err := FetchPullRequests(context.Background(), "octocat", "hello-world", "invalid-state", 0)
		if err != nil {
			t.Fatalf("FetchPullRequests() error = %v", err)
		}
		if !sawGraphQL || !sawREST {
			t.Fatalf("expected GraphQL then REST fallback, sawGraphQL=%v sawREST=%v", sawGraphQL, sawREST)
		}
		if !hasMore || len(prs) != 1 || prs[0].Number != 7 || prs[0].HeadRef != "fix/rest" || prs[0].Author != "alice" {
			t.Fatalf("unexpected REST fallback result: prs=%+v hasMore=%v", prs, hasMore)
		}
	})

	t.Run("graphql ErrNotFound does not fall back to REST", func(t *testing.T) {
		var sawREST bool
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/graphql" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"errors": [{"type": "NOT_FOUND", "message": "Could not resolve to a Repository"}]}`))
				return
			}
			sawREST = true
			w.WriteHeader(http.StatusOK)
		}))

		_, _, err := FetchPullRequests(context.Background(), "octocat", "missing", "open", 1)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		if sawREST {
			t.Fatalf("did not expect REST fallback on ErrNotFound")
		}
	})

	t.Run("page > 1 skips GraphQL and uses REST", func(t *testing.T) {
		var sawGraphQL bool
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/graphql" {
				sawGraphQL = true
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if got := r.URL.Query().Get("page"); got != "2" {
				t.Fatalf("expected page=2, got %q", got)
			}
			if got := r.URL.Query().Get("state"); got != "closed" {
				t.Fatalf("expected state=closed, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"number": 3, "title": "Merged PR", "state": "closed", "merged_at": "2026-11-18T00:00:00Z", "user": {"login": "bob"}, "head": {"ref": "b"}, "base": {"ref": "main"}}]`))
		}))

		prs, hasMore, err := FetchPullRequests(context.Background(), "octocat", "hello-world", "CLOSED", 2)
		if err != nil {
			t.Fatalf("FetchPullRequests() error = %v", err)
		}
		if sawGraphQL {
			t.Fatalf("expected page > 1 to bypass GraphQL")
		}
		if hasMore || len(prs) != 1 || !prs[0].Merged {
			t.Fatalf("unexpected page 2 result: prs=%+v hasMore=%v", prs, hasMore)
		}
	})

	t.Run("invalid owner or repo", func(t *testing.T) {
		if _, _, err := FetchPullRequests(context.Background(), "", "repo", "open", 1); err == nil {
			t.Fatal("expected error for empty owner")
		}
		if _, _, err := FetchPullRequests(context.Background(), "owner/bad", "repo", "open", 1); err == nil {
			t.Fatal("expected error for slash in owner")
		}
	})
}

func TestFetchPullRequestTimeline(t *testing.T) {
	t.Run("merges, filters, and sorts chronologically", func(t *testing.T) {
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/octocat/hello-world/issues/10/comments":
				_, _ = w.Write([]byte(`[
					{"id": 101, "body": "Looks promising!", "author_association": "CONTRIBUTOR", "created_at": "2026-11-20T12:00:00Z", "user": {"login": "carol"}},
					{"id": 102, "body": "   ", "author_association": "NONE", "created_at": "2026-11-20T13:00:00Z", "user": {"login": "spam"}}
				]`))
			case "/repos/octocat/hello-world/pulls/10/reviews":
				_, _ = w.Write([]byte(`[
					{"id": 201, "body": "", "state": "APPROVED", "author_association": "MEMBER", "submitted_at": "2026-11-20T14:00:00Z", "user": {"login": "alice"}},
					{"id": 202, "body": "", "state": "CHANGES_REQUESTED", "author_association": "OWNER", "submitted_at": "2026-11-20T10:30:00Z", "user": {"login": "bob"}},
					{"id": 203, "body": "  ", "state": "COMMENTED", "author_association": "MEMBER", "submitted_at": "2026-11-20T11:00:00Z", "user": {"login": "dave"}}
				]`))
			case "/repos/octocat/hello-world/pulls/10/comments":
				_, _ = w.Write([]byte(`[
					{"id": 301, "body": "Nit: rename var", "path": "main.go", "author_association": "MEMBER", "created_at": "2026-11-20T11:30:00Z", "user": {"login": "alice"}},
					{"id": 302, "body": "", "path": "main.go", "author_association": "MEMBER", "created_at": "2026-11-20T11:45:00Z", "user": {"login": "alice"}}
				]`))
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))

		timeline, err := FetchPullRequestTimeline(context.Background(), "octocat", "hello-world", 10)
		if err != nil {
			t.Fatalf("FetchPullRequestTimeline() error = %v", err)
		}
		if len(timeline) != 4 {
			t.Fatalf("expected 4 timeline items, got %d: %+v", len(timeline), timeline)
		}

		// Expected chronological order:
		// 10:30 review CHANGES_REQUESTED (id 202)
		// 11:30 review_comment (id 301, path main.go)
		// 12:00 issue_comment (id 101)
		// 14:00 review APPROVED (id 201)
		if timeline[0].ID != "202" || timeline[0].Kind != "review" || timeline[0].State != "CHANGES_REQUESTED" {
			t.Fatalf("unexpected item 0: %+v", timeline[0])
		}
		if timeline[1].ID != "301" || timeline[1].Kind != "review_comment" || timeline[1].Path != "main.go" {
			t.Fatalf("unexpected item 1: %+v", timeline[1])
		}
		if timeline[2].ID != "101" || timeline[2].Kind != "issue_comment" || timeline[2].Author != "carol" {
			t.Fatalf("unexpected item 2: %+v", timeline[2])
		}
		if timeline[3].ID != "201" || timeline[3].Kind != "review" || timeline[3].State != "APPROVED" {
			t.Fatalf("unexpected item 3: %+v", timeline[3])
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		if _, err := FetchPullRequestTimeline(context.Background(), "octocat", "hello-world", 0); err == nil {
			t.Fatal("expected error for non-positive PR number")
		}
		if _, err := FetchPullRequestTimeline(context.Background(), "..", "hello-world", 1); err == nil {
			t.Fatal("expected error for invalid owner")
		}
	})
}

func TestFetchIssues(t *testing.T) {
	t.Run("filters out pull requests and parses Link pagination", func(t *testing.T) {
		setupIssuesPRsTestClient(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/octocat/hello-world/issues" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("state"); got != "open" {
				t.Fatalf("expected state=open, got %q", got)
			}
			if got := r.URL.Query().Get("per_page"); got != "30" {
				t.Fatalf("expected per_page=30, got %q", got)
			}
			if got := r.URL.Query().Get("page"); got != "2" {
				t.Fatalf("expected page=2, got %q", got)
			}
			w.Header().Set("Link", `<https://api.github.com/repos/octocat/hello-world/issues?page=3>; rel="next", <https://api.github.com/repos/octocat/hello-world/issues?page=1>; rel="prev"`)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{
					"number": 100,
					"title": "Real issue",
					"body": "Something broke",
					"state": "open",
					"state_reason": "reopened",
					"html_url": "https://github.com/octocat/hello-world/issues/100",
					"comments": 3,
					"created_at": "2026-11-20T08:00:00Z",
					"updated_at": "2026-11-21T08:00:00Z",
					"user": {"login": "octocat"},
					"labels": [{"name": "bug", "color": "d73a4a", "description": "Bug report"}],
					"assignees": [{"login": "alice"}, {"login": "bob"}],
					"pull_request": null
				},
				{
					"number": 101,
					"title": "Actually a PR",
					"body": "PR body",
					"state": "open",
					"html_url": "https://github.com/octocat/hello-world/pull/101",
					"comments": 1,
					"created_at": "2026-11-20T09:00:00Z",
					"updated_at": "2026-11-21T09:00:00Z",
					"user": {"login": "carol"},
					"pull_request": {"url": "https://api.github.com/repos/octocat/hello-world/pulls/101"}
				}
			]`))
		}))

		issues, hasMore, err := FetchIssues(context.Background(), "octocat", "hello-world", "", 2)
		if err != nil {
			t.Fatalf("FetchIssues() error = %v", err)
		}
		if !hasMore {
			t.Fatalf("expected hasMore=true")
		}
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue (PR filtered out), got %d", len(issues))
		}
		got := issues[0]
		if got.Number != 100 || got.Title != "Real issue" || got.StateReason != "reopened" ||
			got.Author != "octocat" || got.CommentCount != 3 ||
			len(got.Labels) != 1 || got.Labels[0].Name != "bug" ||
			len(got.Assignees) != 2 || got.Assignees[0] != "alice" || got.Assignees[1] != "bob" {
			t.Fatalf("unexpected issue: %+v", got)
		}
	})

	t.Run("no next link returns hasMore false", func(t *testing.T) {
		setupIssuesPRsTestClient(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Link", `<https://api.github.com/repos/octocat/hello-world/issues?page=1>; rel="prev"`)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))

		issues, hasMore, err := FetchIssues(context.Background(), "octocat", "hello-world", "closed", 2)
		if err != nil {
			t.Fatalf("FetchIssues() error = %v", err)
		}
		if hasMore || len(issues) != 0 {
			t.Fatalf("expected empty issues and hasMore=false, got len=%d hasMore=%v", len(issues), hasMore)
		}
	})
}

func TestFetchIssueTimeline(t *testing.T) {
	setupIssuesPRsTestClient(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octocat/hello-world/issues/5/comments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Fatalf("expected per_page=100, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": 2, "body": "Second", "author_association": "NONE", "created_at": "2026-11-20T12:00:00Z", "user": {"login": "bob"}},
			{"id": 1, "body": "First", "author_association": "OWNER", "created_at": "2026-11-20T10:00:00Z", "user": {"login": "alice"}}
		]`))
	}))

	comments, err := FetchIssueTimeline(context.Background(), "octocat", "hello-world", 5)
	if err != nil {
		t.Fatalf("FetchIssueTimeline() error = %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].ID != "1" || comments[0].Kind != "issue_comment" || comments[0].Author != "alice" || comments[0].Role != "OWNER" {
		t.Fatalf("unexpected first comment: %+v", comments[0])
	}
	if comments[1].ID != "2" || comments[1].Body != "Second" {
		t.Fatalf("unexpected second comment: %+v", comments[1])
	}
}

func TestCreateIssueComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Date(2026, 11, 25, 14, 0, 0, 0, time.UTC)
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/repos/octocat/hello-world/issues/9/comments" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			var body struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if body.Body != "Hello world!" {
				t.Fatalf("unexpected body: %q", body.Body)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                 999,
				"body":               "Hello world!",
				"author_association": "OWNER",
				"created_at":         now.Format(time.RFC3339),
				"user":               map[string]string{"login": "octocat"},
			})
		}))

		comment, err := CreateIssueComment(context.Background(), "octocat", "hello-world", 9, "Hello world!")
		if err != nil {
			t.Fatalf("CreateIssueComment() error = %v", err)
		}
		if comment.ID != "999" || comment.Author != "octocat" || comment.Body != "Hello world!" || comment.Kind != "issue_comment" || comment.Role != "OWNER" {
			t.Fatalf("unexpected comment: %+v", comment)
		}
	})

	t.Run("empty body rejected without HTTP call", func(t *testing.T) {
		var called bool
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		if _, err := CreateIssueComment(context.Background(), "octocat", "hello-world", 9, "   \n\t "); err == nil {
			t.Fatal("expected error for whitespace-only body")
		}
		if called {
			t.Fatal("expected no HTTP call on empty comment body")
		}
	})

	t.Run("403 forbidden error", func(t *testing.T) {
		setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message": "Resource not accessible by integration"}`))
		}))

		_, err := CreateIssueComment(context.Background(), "octocat", "hello-world", 9, "Test")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestUpdateIssueState(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantState string
		wantErr   bool
	}{
		{name: "close issue", input: "closed", wantState: "closed"},
		{name: "reopen issue uppercase", input: " OPEN ", wantState: "open"},
		{name: "invalid state merged", input: "merged", wantErr: true},
		{name: "empty state", input: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			setupIssuesPRsTestClient(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != http.MethodPatch {
					t.Fatalf("expected PATCH, got %s", r.Method)
				}
				if r.URL.Path != "/repos/octocat/hello-world/issues/12" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				var req struct {
					State string `json:"state"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if req.State != tc.wantState {
					t.Fatalf("expected state %q, got %q", tc.wantState, req.State)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"number": 12,
					"title": "Issue 12",
					"state": "` + tc.wantState + `",
					"user": {"login": "octocat"}
				}`))
			}))

			issue, err := UpdateIssueState(context.Background(), "octocat", "hello-world", 12, tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if called {
					t.Fatal("expected no HTTP request for invalid state")
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateIssueState() error = %v", err)
			}
			if issue.Number != 12 || !strings.EqualFold(issue.State, tc.wantState) {
				t.Fatalf("unexpected issue: %+v", issue)
			}
		})
	}
}
