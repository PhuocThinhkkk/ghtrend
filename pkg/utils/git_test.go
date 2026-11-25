package utils

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestValidateCloneRepo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		owner   string
		repo    string
		wantErr bool
	}{
		{name: "valid simple", owner: "octocat", repo: "hello-world", wantErr: false},
		{name: "valid dots and underscores", owner: "my_org.1", repo: "my.repo_2-beta", wantErr: false},
		{name: "empty owner", owner: "", repo: "repo", wantErr: true},
		{name: "empty repo", owner: "owner", repo: "", wantErr: true},
		{name: "leading dash owner", owner: "-owner", repo: "repo", wantErr: true},
		{name: "leading dash repo", owner: "owner", repo: "--upload-pack=evil", wantErr: true},
		{name: "dot owner", owner: ".", repo: "repo", wantErr: true},
		{name: "dotdot owner", owner: "..", repo: "repo", wantErr: true},
		{name: "dot repo", owner: "owner", repo: ".", wantErr: true},
		{name: "dotdot repo", owner: "owner", repo: "..", wantErr: true},
		{name: "embedded dotdot", owner: "owner", repo: "foo..bar", wantErr: true},
		{name: "slash in owner", owner: "a/b", repo: "repo", wantErr: true},
		{name: "backslash in repo", owner: "owner", repo: `a\b`, wantErr: true},
		{name: "space in repo", owner: "owner", repo: "my repo", wantErr: true},
		{name: "tab in owner", owner: "own\ter", repo: "repo", wantErr: true},
		{name: "non-ascii", owner: "owner", repo: "répô", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCloneRepo(tc.owner, tc.repo)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateCloneRepo(%q, %q) err = %v, wantErr = %v", tc.owner, tc.repo, err, tc.wantErr)
			}
		})
	}
}

func TestRepoCloneURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		owner   string
		repo    string
		want    string
		wantErr bool
	}{
		{
			name:  "valid",
			owner: "octocat",
			repo:  "hello-world",
			want:  "https://github.com/octocat/hello-world.git",
		},
		{
			name:    "invalid",
			owner:   "octocat",
			repo:    "../evil",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := RepoCloneURL(tc.owner, tc.repo)
			if (err != nil) != tc.wantErr {
				t.Fatalf("RepoCloneURL(%q, %q) err = %v, wantErr = %v", tc.owner, tc.repo, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("RepoCloneURL(%q, %q) = %q, want %q", tc.owner, tc.repo, got, tc.want)
			}
		})
	}
}

func TestCloneRepository(t *testing.T) {
	t.Parallel()

	t.Run("constructs safe git clone args", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()

		var gotArgs []string
		runner := func(ctx context.Context, args []string) error {
			if ctx == nil {
				t.Fatalf("expected non-nil ctx")
			}
			gotArgs = slices.Clone(args)
			return nil
		}

		var nilCtx context.Context
		target, err := cloneRepositoryWithRunner(nilCtx, "octocat", "hello-world", parent, runner)
		if err != nil {
			t.Fatalf("cloneRepositoryWithRunner() error = %v", err)
		}

		wantTarget := filepath.Join(parent, "hello-world")
		if target != wantTarget {
			t.Fatalf("target = %q, want %q", target, wantTarget)
		}

		wantArgs := []string{"clone", "--", "https://github.com/octocat/hello-world.git", wantTarget}
		if !slices.Equal(gotArgs, wantArgs) {
			t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
		}
	})

	t.Run("empty parentDir defaults to dot", func(t *testing.T) {
		t.Parallel()
		target, args, err := prepareClone("octocat", "nonexistent-repo-for-test", "   ")
		if err != nil {
			t.Fatalf("prepareClone() error = %v", err)
		}
		wantTarget := filepath.Join(".", "nonexistent-repo-for-test")
		if target != wantTarget {
			t.Fatalf("target = %q, want %q", target, wantTarget)
		}
		wantArgs := []string{"clone", "--", "https://github.com/octocat/nonexistent-repo-for-test.git", wantTarget}
		if !slices.Equal(args, wantArgs) {
			t.Fatalf("args = %v, want %v", args, wantArgs)
		}
	})

	t.Run("existing target directory fails before running git", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		existing := filepath.Join(parent, "hello-world")
		if err := os.MkdirAll(existing, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		called := false
		runner := func(ctx context.Context, args []string) error {
			called = true
			return nil
		}

		_, err := cloneRepositoryWithRunner(context.Background(), "octocat", "hello-world", parent, runner)
		if err == nil {
			t.Fatalf("expected error when target directory already exists")
		}
		if called {
			t.Fatalf("runner should not be called when target directory exists")
		}
	})

	t.Run("invalid slug fails before running git", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		called := false
		runner := func(ctx context.Context, args []string) error {
			called = true
			return nil
		}

		_, err := cloneRepositoryWithRunner(context.Background(), "octocat", "-bad", parent, runner)
		if err == nil {
			t.Fatalf("expected error for invalid repo slug")
		}
		if called {
			t.Fatalf("runner should not be called for invalid repo slug")
		}
	})

	t.Run("runner error is propagated", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		wantErr := errors.New("git exit status 128")
		runner := func(ctx context.Context, args []string) error {
			return wantErr
		}

		_, err := cloneRepositoryWithRunner(context.Background(), "octocat", "hello-world", parent, runner)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	})
}
