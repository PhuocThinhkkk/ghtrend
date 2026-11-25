package ghclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func setupWorkspaceTestClient(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(handler)
	defaultClientMu.Lock()
	origClient := DefaultClient
	DefaultClient = &Client{
		APIBaseURL: srv.URL,
		GraphQLURL: srv.URL + "/graphql",
		WebBaseURL: srv.URL,
		HTTPClient: srv.Client(),
		TokenFunc:  func() string { return "" },
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

	return srv
}

func TestFetchRepoMetadata(t *testing.T) {
	t.Run("success with full fields", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/octocat/hello-world" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"owner": {"login": "octocat"},
				"name": "hello-world",
				"full_name": "octocat/hello-world",
				"description": "My first repository",
				"default_branch": "main",
				"license": {"spdx_id": "MIT", "name": "MIT License"},
				"language": "Go",
				"html_url": "https://github.com/octocat/hello-world",
				"stargazers_count": 1200,
				"forks_count": 340,
				"watchers_count": 1200,
				"subscribers_count": 88,
				"open_issues_count": 12,
				"topics": ["go", "cli", "tui"],
				"private": false,
				"fork": true,
				"archived": true
			}`))
		}))

		meta, err := FetchRepoMetadata(context.Background(), "octocat", "hello-world")
		if err != nil {
			t.Fatalf("FetchRepoMetadata() error = %v", err)
		}

		if meta.Owner != "octocat" || meta.Name != "hello-world" || meta.FullName != "octocat/hello-world" {
			t.Fatalf("unexpected identity fields: %+v", meta)
		}
		if meta.DefaultBranch != "main" || meta.License != "MIT" || meta.Language != "Go" {
			t.Fatalf("unexpected repo details: %+v", meta)
		}
		if meta.Stars != 1200 || meta.Forks != 340 || meta.Watchers != 88 || meta.OpenIssues != 12 {
			t.Fatalf("unexpected counts: %+v", meta)
		}
		if !meta.IsFork || !meta.IsArchived || meta.IsPrivate {
			t.Fatalf("unexpected flags: %+v", meta)
		}
		if len(meta.Topics) != 3 || meta.Topics[0] != "go" {
			t.Fatalf("unexpected topics: %+v", meta.Topics)
		}
	})

	t.Run("license fallback and watchers fallback", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"name": "repo",
				"full_name": "org/repo",
				"license": {"spdx_id": "NOASSERTION", "name": "Custom License"},
				"watchers_count": 42
			}`))
		}))

		meta, err := FetchRepoMetadata(context.Background(), "org", "repo")
		if err != nil {
			t.Fatalf("FetchRepoMetadata() error = %v", err)
		}
		if meta.Owner != "org" {
			t.Fatalf("expected fallback owner org, got %q", meta.Owner)
		}
		if meta.License != "Custom License" {
			t.Fatalf("expected Custom License, got %q", meta.License)
		}
		if meta.Watchers != 42 {
			t.Fatalf("expected Watchers 42, got %d", meta.Watchers)
		}
	})

	t.Run("not found", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		}))

		_, err := FetchRepoMetadata(context.Background(), "octocat", "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("invalid owner or repo", func(t *testing.T) {
		if _, err := FetchRepoMetadata(context.Background(), "", "repo"); err == nil {
			t.Fatal("expected error for empty owner")
		}
		if _, err := FetchRepoMetadata(context.Background(), "owner", ".."); err == nil {
			t.Fatal("expected error for .. repo")
		}
	})
}

func TestFetchRepoRefs(t *testing.T) {
	cases := []struct {
		name         string
		branchStatus int
		branchBody   string
		tagStatus    int
		tagBody      string
		wantErr      bool
		wantCount    int
		wantKinds    []string
	}{
		{
			name:         "branches and tags succeed",
			branchStatus: http.StatusOK,
			branchBody:   `[{"name":"main","commit":{"sha":"aaa111"}},{"name":"dev","commit":{"sha":"bbb222"}}]`,
			tagStatus:    http.StatusOK,
			tagBody:      `[{"name":"v1.0.0","commit":{"sha":"ccc333"}}]`,
			wantCount:    3,
			wantKinds:    []string{"branch", "branch", "tag"},
		},
		{
			name:         "branches succeed tags fail",
			branchStatus: http.StatusOK,
			branchBody:   `[{"name":"main","commit":{"sha":"aaa111"}}]`,
			tagStatus:    http.StatusInternalServerError,
			tagBody:      `{"message":"server error"}`,
			wantCount:    1,
			wantKinds:    []string{"branch"},
		},
		{
			name:         "branches fail tags succeed",
			branchStatus: http.StatusInternalServerError,
			branchBody:   `{"message":"server error"}`,
			tagStatus:    http.StatusOK,
			tagBody:      `[{"name":"v1.0.0","commit":{"sha":"ccc333"}}]`,
			wantCount:    1,
			wantKinds:    []string{"tag"},
		},
		{
			name:         "both fail returns combined error",
			branchStatus: http.StatusNotFound,
			branchBody:   `{"message":"Not Found"}`,
			tagStatus:    http.StatusInternalServerError,
			tagBody:      `{"message":"boom"}`,
			wantErr:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("per_page") != "100" {
					t.Fatalf("expected per_page=100, got %q", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/repos/octocat/hello/branches":
					w.WriteHeader(tc.branchStatus)
					_, _ = w.Write([]byte(tc.branchBody))
				case "/repos/octocat/hello/tags":
					w.WriteHeader(tc.tagStatus)
					_, _ = w.Write([]byte(tc.tagBody))
				default:
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
			}))

			refs, err := FetchRepoRefs(context.Background(), "octocat", "hello")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "fetching branches") || !strings.Contains(err.Error(), "fetching tags") {
					t.Fatalf("expected combined error message, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(refs) != tc.wantCount {
				t.Fatalf("expected %d refs, got %d: %+v", tc.wantCount, len(refs), refs)
			}
			for i, kind := range tc.wantKinds {
				if refs[i].Kind != kind {
					t.Fatalf("refs[%d].Kind = %q, want %q", i, refs[i].Kind, kind)
				}
			}
		})
	}
}

func TestFetchRepoTree(t *testing.T) {
	t.Run("sorted directories before files and escaped ref", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.EscapedPath() != "/repos/octocat/hello/git/trees/feat%2Fnew-ui" {
				t.Fatalf("unexpected escaped path: %s", r.URL.EscapedPath())
			}
			if r.URL.Query().Get("recursive") != "1" {
				t.Fatalf("expected recursive=1, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"sha": "rootsha",
				"truncated": true,
				"tree": [
					{"path": "README.md", "mode": "100644", "type": "blob", "sha": "s1", "size": 120},
					{"path": "src", "mode": "040000", "type": "tree", "sha": "s2", "size": 0},
					{"path": "cmd/app.go", "mode": "100644", "type": "blob", "sha": "s3", "size": 450},
					{"path": "Cmd", "mode": "040000", "type": "tree", "sha": "s4", "size": 0}
				]
			}`))
		}))

		entries, truncated, err := FetchRepoTree(context.Background(), "octocat", "hello", "feat/new-ui")
		if err != nil {
			t.Fatalf("FetchRepoTree() error = %v", err)
		}
		if !truncated {
			t.Fatal("expected truncated=true")
		}
		wantOrder := []string{"Cmd", "src", "cmd/app.go", "README.md"}
		wantNames := []string{"Cmd", "src", "app.go", "README.md"}
		if len(entries) != len(wantOrder) {
			t.Fatalf("got %d entries, want %d", len(entries), len(wantOrder))
		}
		for i := range wantOrder {
			if entries[i].Path != wantOrder[i] {
				t.Fatalf("entries[%d].Path = %q, want %q", i, entries[i].Path, wantOrder[i])
			}
			if entries[i].Name != wantNames[i] {
				t.Fatalf("entries[%d].Name = %q, want %q", i, entries[i].Name, wantNames[i])
			}
		}
	})

	t.Run("empty repository returns nil false nil on 409", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message": "Git Repository is empty."}`))
		}))

		entries, truncated, err := FetchRepoTree(context.Background(), "octocat", "empty", "main")
		if err != nil {
			t.Fatalf("expected nil error on 409, got %v", err)
		}
		if truncated {
			t.Fatal("expected truncated=false")
		}
		if entries != nil {
			t.Fatalf("expected nil entries, got %+v", entries)
		}
	})

	t.Run("empty ref rejected", func(t *testing.T) {
		if _, _, err := FetchRepoTree(context.Background(), "octocat", "repo", ""); err == nil {
			t.Fatal("expected error for empty ref")
		}
	})
}

func TestFetchRepoDirContents(t *testing.T) {
	t.Run("nested dir with escaping and sorting", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.EscapedPath() != "/repos/octocat/hello/contents/docs/my%20guide" {
				t.Fatalf("unexpected escaped path: %s", r.URL.EscapedPath())
			}
			if r.URL.Query().Get("ref") != "v1.0" {
				t.Fatalf("expected ref=v1.0, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "zebra.md", "path": "docs/my guide/zebra.md", "sha": "1", "size": 10, "type": "file"},
				{"name": "assets", "path": "docs/my guide/assets", "sha": "2", "size": 0, "type": "dir"},
				{"name": "Alpha.md", "path": "docs/my guide/Alpha.md", "sha": "3", "size": 20, "type": "file"},
			})
		}))

		entries, err := FetchRepoDirContents(context.Background(), "octocat", "hello", "/docs/my guide/", "v1.0")
		if err != nil {
			t.Fatalf("FetchRepoDirContents() error = %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		if entries[0].Name != "assets" || entries[0].Type != "tree" {
			t.Fatalf("expected first entry to be dir assets, got %+v", entries[0])
		}
		if entries[1].Name != "Alpha.md" || entries[1].Type != "blob" {
			t.Fatalf("expected second entry Alpha.md, got %+v", entries[1])
		}
		if entries[2].Name != "zebra.md" || entries[2].Type != "blob" {
			t.Fatalf("expected third entry zebra.md, got %+v", entries[2])
		}
	})

	t.Run("root dir without ref", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/octocat/hello/contents" || r.URL.RawQuery != "" {
				t.Fatalf("unexpected URL: %s", r.URL.String())
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))

		entries, err := FetchRepoDirContents(context.Background(), "octocat", "hello", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})
}

func TestFetchFileContent(t *testing.T) {
	t.Run("text file", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Accept"); got != "application/vnd.github.raw+json" {
				t.Fatalf("unexpected Accept header: %q", got)
			}
			if r.URL.EscapedPath() != "/repos/octocat/hello/contents/pkg/my%20file.go" {
				t.Fatalf("unexpected escaped path: %s", r.URL.EscapedPath())
			}
			if r.URL.Query().Get("ref") != "main" {
				t.Fatalf("expected ref=main, got %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte("package main\n"))
		}))

		fc, err := FetchFileContent(context.Background(), "octocat", "hello", "pkg/my file.go", "main")
		if err != nil {
			t.Fatalf("FetchFileContent() error = %v", err)
		}
		if fc.Content != "package main\n" || fc.Size != 13 || fc.IsBinary || fc.Truncated {
			t.Fatalf("unexpected FileContent: %+v", fc)
		}
		if fc.Path != "pkg/my file.go" || fc.Ref != "main" {
			t.Fatalf("unexpected path/ref: %+v", fc)
		}
	})

	t.Run("truncates over 512 KiB on valid UTF-8 boundary", func(t *testing.T) {
		// Place a 3-byte UTF-8 rune ("€" = 0xE2 0x82 0xAC) straddling the 512 KiB boundary.
		prefix := strings.Repeat("a", maxWorkspaceFileBytes-1)
		payload := prefix + "€tail"

		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(payload))
		}))

		fc, err := FetchFileContent(context.Background(), "octocat", "hello", "big.txt", "main")
		if err != nil {
			t.Fatalf("FetchFileContent() error = %v", err)
		}
		if !fc.Truncated || fc.IsBinary {
			t.Fatalf("expected Truncated=true IsBinary=false, got %+v", fc)
		}
		if fc.Size != int64(len(payload)) {
			t.Fatalf("Size = %d, want %d", fc.Size, len(payload))
		}
		if len(fc.Content) != maxWorkspaceFileBytes-1 {
			t.Fatalf("len(Content) = %d, want %d", len(fc.Content), maxWorkspaceFileBytes-1)
		}
		if !utf8.ValidString(fc.Content) {
			t.Fatal("truncated content is not valid UTF-8")
		}
	})

	t.Run("binary file with nul byte", func(t *testing.T) {
		raw := []byte{0x89, 'P', 'N', 'G', 0x00, 0x01}
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(raw)
		}))

		fc, err := FetchFileContent(context.Background(), "octocat", "hello", "logo.png", "main")
		if err != nil {
			t.Fatalf("FetchFileContent() error = %v", err)
		}
		if !fc.IsBinary || fc.Content != "" || fc.Size != int64(len(raw)) {
			t.Fatalf("unexpected binary FileContent: %+v", fc)
		}
	})

	t.Run("invalid utf8 treated as binary", func(t *testing.T) {
		raw := []byte{0xff, 0xfe, 0xfd}
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(raw)
		}))

		fc, err := FetchFileContent(context.Background(), "octocat", "hello", "data.bin", "main")
		if err != nil {
			t.Fatalf("FetchFileContent() error = %v", err)
		}
		if !fc.IsBinary || fc.Content != "" {
			t.Fatalf("expected binary FileContent, got %+v", fc)
		}
	})

	t.Run("empty or traversal path rejected", func(t *testing.T) {
		if _, err := FetchFileContent(context.Background(), "octocat", "hello", "", "main"); err == nil {
			t.Fatal("expected error for empty path")
		}
		if _, err := FetchFileContent(context.Background(), "octocat", "hello", "../secret", "main"); err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

func TestFetchRepoREADME(t *testing.T) {
	t.Run("success with ref", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Accept"); got != "application/vnd.github.raw+json" {
				t.Fatalf("unexpected Accept header: %q", got)
			}
			if r.URL.Path != "/repos/octocat/hello/readme" || r.URL.Query().Get("ref") != "v2.0" {
				t.Fatalf("unexpected URL: %s", r.URL.String())
			}
			_, _ = w.Write([]byte("# Hello World\n"))
		}))

		readme, err := FetchRepoREADME(context.Background(), "octocat", "hello", "v2.0")
		if err != nil {
			t.Fatalf("FetchRepoREADME() error = %v", err)
		}
		if readme != "# Hello World\n" {
			t.Fatalf("unexpected readme: %q", readme)
		}
	})

	t.Run("missing readme returns empty string and nil error", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		}))

		readme, err := FetchRepoREADME(context.Background(), "octocat", "hello", "")
		if err != nil {
			t.Fatalf("expected nil error on 404, got %v", err)
		}
		if readme != "" {
			t.Fatalf("expected empty readme, got %q", readme)
		}
	})

	t.Run("server error propagates", func(t *testing.T) {
		setupWorkspaceTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
		}))

		if _, err := FetchRepoREADME(context.Background(), "octocat", "hello", "main"); err == nil {
			t.Fatal("expected error on 500")
		}
	})
}

func TestEscapeRepoPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "root slash", input: "/", want: ""},
		{name: "simple", input: "pkg/ghclient/workspace.go", want: "pkg/ghclient/workspace.go"},
		{name: "spaces and special chars", input: "/docs/my guide/hello#1.md/", want: "docs/my%20guide/hello%231.md"},
		{name: "dotdot rejected", input: "pkg/../secret", wantErr: true},
		{name: "dot rejected", input: "./file.go", wantErr: true},
		{name: "backslash rejected", input: `pkg\file.go`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := escapeRepoPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
