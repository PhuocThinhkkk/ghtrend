package ghclient

import (
	"context"
	"net/http"
	"testing"
)

func TestSearchRepositories(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			query string
		}{
			{name: "empty", query: ""},
			{name: "whitespace", query: "   \t\n  "},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := SearchRepositories(context.Background(), tc.query, 1)
				if err == nil {
					t.Fatalf("expected error for empty query")
				}
			})
		}
	})

	t.Run("success with total_count pagination", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/search/repositories" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("q") != "bubbletea language:go" || q.Get("per_page") != "15" || q.Get("page") != "1" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"total_count": 20,
				"items": [
					{
						"full_name": "charmbracelet/bubbletea",
						"name": "bubbletea",
						"owner": {"login": "charmbracelet"},
						"description": "A powerful little TUI framework",
						"language": "Go",
						"stargazers_count": 30000,
						"forks_count": 900,
						"html_url": "https://github.com/charmbracelet/bubbletea"
					},
					{
						"full_name": "anon/repo",
						"name": "repo",
						"owner": null,
						"description": "",
						"language": "",
						"stargazers_count": 1,
						"forks_count": 0,
						"html_url": "https://github.com/anon/repo"
					}
				]
			}`))
		}))

		var nilCtx context.Context
		results, hasNext, err := SearchRepositories(nilCtx, "bubbletea language:go", 0)
		if err != nil {
			t.Fatalf("SearchRepositories() error = %v", err)
		}
		if !hasNext {
			t.Fatalf("expected hasNext = true when total_count > 15")
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].FullName != "charmbracelet/bubbletea" || results[0].Owner != "charmbracelet" || results[0].Stars != 30000 {
			t.Fatalf("unexpected first result: %+v", results[0])
		}
		if results[1].Owner != "" {
			t.Fatalf("expected empty owner for nil owner object, got %q", results[1].Owner)
		}
	})

	t.Run("hasNext via Link header and false when exhausted", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			w.Header().Set("Content-Type", "application/json")
			if page == "1" {
				w.Header().Set("Link", `<https://api.github.com/search/repositories?q=go&page=2>; rel="next"`)
			}
			_, _ = w.Write([]byte(`{"total_count": 10, "items": []}`))
		}))

		_, hasNext1, err := SearchRepositories(context.Background(), "go", 1)
		if err != nil || !hasNext1 {
			t.Fatalf("page 1: hasNext=%v err=%v, want true, nil", hasNext1, err)
		}

		_, hasNext2, err := SearchRepositories(context.Background(), "go", 2)
		if err != nil || hasNext2 {
			t.Fatalf("page 2: hasNext=%v err=%v, want false, nil", hasNext2, err)
		}
	})

	t.Run("api error", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
		}))

		_, _, err := SearchRepositories(context.Background(), "invalid:", 1)
		if err == nil {
			t.Fatalf("expected error on 422 response")
		}
	})
}

func TestSearchUsers(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			query string
		}{
			{name: "empty", query: ""},
			{name: "whitespace", query: "  \n "},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := SearchUsers(context.Background(), tc.query, 1)
				if err == nil {
					t.Fatalf("expected error for empty query")
				}
			})
		}
	})

	t.Run("success and pagination", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/search/users" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("q") != "octocat" || q.Get("per_page") != "15" || q.Get("page") != "2" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"total_count": 30,
				"items": [
					{
						"login": "octocat",
						"name": "The Octocat",
						"avatar_url": "https://avatars.githubusercontent.com/u/583231",
						"html_url": "https://github.com/octocat",
						"type": "User"
					}
				]
			}`))
		}))

		var nilCtx context.Context
		results, hasNext, err := SearchUsers(nilCtx, "octocat", 2)
		if err != nil {
			t.Fatalf("SearchUsers() error = %v", err)
		}
		if hasNext {
			t.Fatalf("expected hasNext = false when total_count == 30 on page 2")
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Login != "octocat" || results[0].Name != "The Octocat" || results[0].Type != "User" {
			t.Fatalf("unexpected user result: %+v", results[0])
		}
	})

	t.Run("hasNext via total_count and Link header", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"total_count": 31, "items": []}`))
		}))

		_, hasNext, err := SearchUsers(context.Background(), "go", 2)
		if err != nil || !hasNext {
			t.Fatalf("expected hasNext=true, err=nil; got %v, %v", hasNext, err)
		}
	})
}
