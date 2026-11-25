package ghclient

import (
	"context"
	"net/http"
	"testing"
)

func TestIsRepoStarred(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			owner string
			repo  string
		}{
			{name: "empty owner", owner: "", repo: "repo"},
			{name: "empty repo", owner: "owner", repo: ""},
			{name: "dotdot", owner: "..", repo: "repo"},
			{name: "slash", owner: "owner", repo: "a/b"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := IsRepoStarred(context.Background(), tc.owner, tc.repo)
				if err == nil {
					t.Fatalf("expected validation error, got nil")
				}
			})
		}
	})

	t.Run("starred 204", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/user/starred/octocat/hello-world" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		var nilCtx context.Context
		starred, err := IsRepoStarred(nilCtx, "octocat", "hello-world")
		if err != nil {
			t.Fatalf("IsRepoStarred() error = %v", err)
		}
		if !starred {
			t.Fatalf("expected starred = true")
		}
	})

	t.Run("not starred 404", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		}))

		starred, err := IsRepoStarred(context.Background(), "octocat", "hello-world")
		if err != nil {
			t.Fatalf("IsRepoStarred() error = %v, want nil on 404", err)
		}
		if starred {
			t.Fatalf("expected starred = false")
		}
	})

	t.Run("unexpected server error", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Server Error"}`))
		}))

		starred, err := IsRepoStarred(context.Background(), "octocat", "hello-world")
		if err == nil {
			t.Fatalf("expected error on 500 response")
		}
		if starred {
			t.Fatalf("expected starred = false on error")
		}
	})
}

func TestSetRepoStarred(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			owner string
			repo  string
		}{
			{name: "empty owner", owner: "", repo: "repo"},
			{name: "empty repo", owner: "owner", repo: ""},
			{name: "backslash", owner: `a\b`, repo: "repo"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := SetRepoStarred(context.Background(), tc.owner, tc.repo, true); err == nil {
					t.Fatalf("expected validation error, got nil")
				}
			})
		}
	})

	t.Run("star and unstar methods", func(t *testing.T) {
		cases := []struct {
			name       string
			star       bool
			wantMethod string
		}{
			{name: "star uses PUT", star: true, wantMethod: http.MethodPut},
			{name: "unstar uses DELETE", star: false, wantMethod: http.MethodDelete},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var gotMethod string
				setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotMethod = r.Method
					if r.URL.Path != "/user/starred/octocat/hello-world" {
						t.Fatalf("unexpected path: %s", r.URL.Path)
					}
					w.WriteHeader(http.StatusNoContent)
				}))

				var nilCtx context.Context
				if err := SetRepoStarred(nilCtx, "octocat", "hello-world", tc.star); err != nil {
					t.Fatalf("SetRepoStarred() error = %v", err)
				}
				if gotMethod != tc.wantMethod {
					t.Fatalf("method = %s, want %s", gotMethod, tc.wantMethod)
				}
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Requires authentication"}`))
		}))

		if err := SetRepoStarred(context.Background(), "octocat", "hello-world", true); err == nil {
			t.Fatalf("expected error on 401 response")
		}
	})
}
