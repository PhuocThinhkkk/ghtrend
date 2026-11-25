package ghclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestFetchRepoCommits(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			owner string
			repo  string
		}{
			{name: "empty owner", owner: "", repo: "repo"},
			{name: "empty repo", owner: "owner", repo: ""},
			{name: "whitespace owner", owner: "   ", repo: "repo"},
			{name: "dotdot owner", owner: "..", repo: "repo"},
			{name: "slash in repo", owner: "owner", repo: "a/b"},
			{name: "backslash in owner", owner: `a\b`, repo: "repo"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := FetchRepoCommits(context.Background(), tc.owner, tc.repo, "", 1)
				if err == nil {
					t.Fatalf("expected validation error, got nil")
				}
			})
		}
	})

	t.Run("success with ref and pagination", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.URL.Path != "/repos/octocat/hello-world/commits" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("per_page") != "30" || q.Get("page") != "2" || q.Get("sha") != "feat/new" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Link", `<https://api.github.com/repos/octocat/hello-world/commits?page=3>; rel="next"`)
			_, _ = w.Write([]byte(`[
				{
					"sha": "1234567890abcdef1234567890abcdef12345678",
					"html_url": "https://github.com/octocat/hello-world/commit/1234567890abcdef",
					"commit": {
						"message": "feat: add commits\r\n\r\nDetailed body line",
						"author": {"name": "Octo Cat", "date": "2026-01-02T03:04:05Z"},
						"committer": {"name": "GitHub", "date": "2026-01-02T03:05:00Z"}
					},
					"author": {"login": "octocat"}
				},
				{
					"sha": "abc12",
					"html_url": "https://github.com/octocat/hello-world/commit/abc12",
					"commit": {
						"message": "fix: nil author",
						"author": {"name": "Committer Only", "date": "0001-01-01T00:00:00Z"},
						"committer": {"name": "Fallback", "date": "2026-02-03T04:05:06Z"}
					},
					"author": null
				}
			]`))
		}))

		commits, hasNext, err := FetchRepoCommits(context.Background(), "octocat", "hello-world", "feat/new", 2)
		if err != nil {
			t.Fatalf("FetchRepoCommits() error = %v", err)
		}
		if !hasNext {
			t.Fatalf("expected hasNext = true")
		}
		if len(commits) != 2 {
			t.Fatalf("expected 2 commits, got %d", len(commits))
		}

		c0 := commits[0]
		if c0.ShortSHA != "1234567" {
			t.Fatalf("unexpected ShortSHA: %q", c0.ShortSHA)
		}
		if c0.Message != "feat: add commits" {
			t.Fatalf("unexpected Message: %q", c0.Message)
		}
		if c0.Body != "feat: add commits\r\n\r\nDetailed body line" {
			t.Fatalf("unexpected Body: %q", c0.Body)
		}
		if c0.Author != "Octo Cat" || c0.AuthorLogin != "octocat" {
			t.Fatalf("unexpected author fields: %+v", c0)
		}
		if !c0.Date.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
			t.Fatalf("unexpected Date: %v", c0.Date)
		}

		c1 := commits[1]
		if c1.ShortSHA != "abc12" || c1.Author != "Committer Only" || c1.AuthorLogin != "" {
			t.Fatalf("unexpected second commit: %+v", c1)
		}
		if !c1.Date.Equal(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)) {
			t.Fatalf("expected committer date fallback, got %v", c1.Date)
		}
	})

	t.Run("nil context, default page, empty ref, no next page", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("page") != "1" {
				t.Fatalf("expected page=1, got %q", q.Get("page"))
			}
			if q.Has("sha") {
				t.Fatalf("expected no sha query param, got %q", q.Get("sha"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))

		var nilCtx context.Context
		commits, hasNext, err := FetchRepoCommits(nilCtx, "octocat", "hello-world", "   ", 0)
		if err != nil {
			t.Fatalf("FetchRepoCommits() error = %v", err)
		}
		if hasNext {
			t.Fatalf("expected hasNext = false")
		}
		if len(commits) != 0 {
			t.Fatalf("expected 0 commits, got %d", len(commits))
		}
	})

	t.Run("not found error", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		}))

		_, _, err := FetchRepoCommits(context.Background(), "octocat", "missing", "", 1)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestFetchCommitDiff(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			owner string
			repo  string
			sha   string
		}{
			{name: "empty owner", owner: "", repo: "repo", sha: "abc1234"},
			{name: "empty sha", owner: "owner", repo: "repo", sha: ""},
			{name: "whitespace sha", owner: "owner", repo: "repo", sha: "   "},
			{name: "space in sha", owner: "owner", repo: "repo", sha: "abc 123"},
			{name: "slash in sha", owner: "owner", repo: "repo", sha: "abc/123"},
			{name: "backslash in sha", owner: "owner", repo: "repo", sha: `abc\123`},
			{name: "dotdot in sha", owner: "owner", repo: "repo", sha: "abc..123"},
			{name: "single dot sha", owner: "owner", repo: "repo", sha: "."},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := FetchCommitDiff(context.Background(), tc.owner, tc.repo, tc.sha)
				if err == nil {
					t.Fatalf("expected validation error, got nil")
				}
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/octocat/hello-world/commits/1234567890abcdef" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"sha": "1234567890abcdef1234567890abcdef12345678",
				"html_url": "https://github.com/octocat/hello-world/commit/1234567890abcdef",
				"commit": {
					"message": "refactor: rename file\n\nMoved to new location",
					"author": {"name": "", "date": "2026-03-04T05:06:07Z"}
				},
				"author": {"login": "octocat"},
				"stats": {"additions": 10, "deletions": 3},
				"files": [
					{
						"filename": "pkg/new.go",
						"status": "renamed",
						"additions": 10,
						"deletions": 3,
						"changes": 13,
						"patch": "@@ -1,3 +1,10 @@",
						"previous_filename": "pkg/old.go"
					}
				]
			}`))
		}))

		var nilCtx context.Context
		diff, err := FetchCommitDiff(nilCtx, "octocat", "hello-world", "1234567890abcdef")
		if err != nil {
			t.Fatalf("FetchCommitDiff() error = %v", err)
		}
		if diff.ShortSHA != "1234567" || diff.Author != "octocat" || diff.AuthorLogin != "octocat" {
			t.Fatalf("unexpected diff metadata: %+v", diff)
		}
		if diff.Additions != 10 || diff.Deletions != 3 || diff.ChangedFiles != 1 {
			t.Fatalf("unexpected diff stats: %+v", diff)
		}
		if len(diff.Files) != 1 || diff.Files[0].PreviousFilename != "pkg/old.go" || diff.Files[0].Status != "renamed" {
			t.Fatalf("unexpected diff files: %+v", diff.Files)
		}
	})

	t.Run("not found", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No commit found for SHA"}`))
		}))

		_, err := FetchCommitDiff(context.Background(), "octocat", "hello-world", "deadbeef")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
