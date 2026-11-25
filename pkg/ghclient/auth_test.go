package ghclient

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveAuthTokenWith(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      map[string]string
		cliFn    func(ctx context.Context) (string, error)
		want     string
		wantCLI  bool
		checkCtx bool
	}{
		{
			name: "GITHUB_TOKEN takes precedence over GH_TOKEN and CLI",
			env: map[string]string{
				"GITHUB_TOKEN": "  ghp_primary  ",
				"GH_TOKEN":     "ghp_secondary",
			},
			cliFn: func(ctx context.Context) (string, error) {
				return "ghp_cli", nil
			},
			want:    "ghp_primary",
			wantCLI: false,
		},
		{
			name: "GH_TOKEN used when GITHUB_TOKEN empty or whitespace",
			env: map[string]string{
				"GITHUB_TOKEN": "   ",
				"GH_TOKEN":     " ghp_secondary\n",
			},
			cliFn: func(ctx context.Context) (string, error) {
				return "ghp_cli", nil
			},
			want:    "ghp_secondary",
			wantCLI: false,
		},
		{
			name: "falls back to CLI when env vars unset",
			env:  map[string]string{},
			cliFn: func(ctx context.Context) (string, error) {
				return " gho_from_cli\r\n", nil
			},
			want:     "gho_from_cli",
			wantCLI:  true,
			checkCtx: true,
		},
		{
			name: "returns empty on CLI error",
			env:  map[string]string{},
			cliFn: func(ctx context.Context) (string, error) {
				return "", errors.New("gh not logged in")
			},
			want:    "",
			wantCLI: true,
		},
		{
			name:    "nil CLI callback returns empty",
			env:     map[string]string{},
			cliFn:   nil,
			want:    "",
			wantCLI: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var cliCalled atomic.Bool
			var hasDeadline atomic.Bool
			var remainingDeadline time.Duration

			getenv := func(key string) string {
				return tc.env[key]
			}

			var wrappedCLI func(context.Context) (string, error)
			if tc.cliFn != nil {
				wrappedCLI = func(ctx context.Context) (string, error) {
					cliCalled.Store(true)
					if deadline, ok := ctx.Deadline(); ok {
						hasDeadline.Store(true)
						remainingDeadline = time.Until(deadline)
					}
					return tc.cliFn(ctx)
				}
			}

			got := ResolveAuthTokenWith(getenv, wrappedCLI)
			if got != tc.want {
				t.Fatalf("ResolveAuthTokenWith() = %q, want %q", got, tc.want)
			}
			if cliCalled.Load() != tc.wantCLI {
				t.Fatalf("cliCalled = %v, want %v", cliCalled.Load(), tc.wantCLI)
			}
			if tc.checkCtx {
				if !hasDeadline.Load() {
					t.Fatal("expected CLI context to carry a deadline")
				}
				if remainingDeadline <= 0 || remainingDeadline > 3*time.Second {
					t.Fatalf("expected ~2s deadline, got %v", remainingDeadline)
				}
			}
		})
	}
}

func TestAuthResolver_CachingAndLiveEnvPrecedence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	env := map[string]string{}
	var cliCalls atomic.Int32

	resolver := &AuthResolver{
		failCooldown: 30 * time.Second,
		now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		},
	}

	getenv := func(k string) string {
		mu.Lock()
		defer mu.Unlock()
		return env[k]
	}
	cliFn := func(ctx context.Context) (string, error) {
		cliCalls.Add(1)
		return "cli_cached_token", nil
	}

	// 1. Cold start resolves from CLI and caches it.
	if got := resolver.Resolve(getenv, cliFn); got != "cli_cached_token" {
		t.Fatalf("first resolve = %q, want cli_cached_token", got)
	}
	// 2. Subsequent call hits cache without invoking CLI again.
	if got := resolver.Resolve(getenv, cliFn); got != "cli_cached_token" {
		t.Fatalf("second resolve = %q, want cli_cached_token", got)
	}
	if calls := cliCalls.Load(); calls != 1 {
		t.Fatalf("cliCalls = %d, want 1", calls)
	}

	// 3. Setting GH_TOKEN overrides cached CLI token immediately.
	mu.Lock()
	env["GH_TOKEN"] = "live_gh_token"
	mu.Unlock()
	if got := resolver.Resolve(getenv, cliFn); got != "live_gh_token" {
		t.Fatalf("with GH_TOKEN = %q, want live_gh_token", got)
	}

	// 4. Setting GITHUB_TOKEN overrides GH_TOKEN immediately.
	mu.Lock()
	env["GITHUB_TOKEN"] = "live_github_token"
	mu.Unlock()
	if got := resolver.Resolve(getenv, cliFn); got != "live_github_token" {
		t.Fatalf("with GITHUB_TOKEN = %q, want live_github_token", got)
	}

	// 5. Clearing env vars falls back to cached CLI token without extra CLI calls.
	mu.Lock()
	delete(env, "GITHUB_TOKEN")
	delete(env, "GH_TOKEN")
	mu.Unlock()
	if got := resolver.Resolve(getenv, cliFn); got != "cli_cached_token" {
		t.Fatalf("after clearing env = %q, want cli_cached_token", got)
	}
	if calls := cliCalls.Load(); calls != 1 {
		t.Fatalf("cliCalls = %d, want 1 after env changes", calls)
	}
}

func TestAuthResolver_FailureCooldown(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	curTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	shouldFail := true
	var cliCalls atomic.Int32

	resolver := &AuthResolver{
		failCooldown: 30 * time.Second,
		now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return curTime
		},
	}

	getenv := func(string) string { return "" }
	cliFn := func(ctx context.Context) (string, error) {
		cliCalls.Add(1)
		mu.Lock()
		fail := shouldFail
		mu.Unlock()
		if fail {
			return "", errors.New("temporary failure")
		}
		return "recovered_token", nil
	}

	// Initial failure invokes CLI once and returns "".
	if got := resolver.Resolve(getenv, cliFn); got != "" {
		t.Fatalf("got %q on failure, want empty", got)
	}
	if calls := cliCalls.Load(); calls != 1 {
		t.Fatalf("cliCalls = %d, want 1", calls)
	}

	// Within 30s cooldown, CLI is NOT invoked again.
	mu.Lock()
	curTime = curTime.Add(20 * time.Second)
	mu.Unlock()
	if got := resolver.Resolve(getenv, cliFn); got != "" {
		t.Fatalf("got %q during cooldown, want empty", got)
	}
	if calls := cliCalls.Load(); calls != 1 {
		t.Fatalf("cliCalls during cooldown = %d, want 1", calls)
	}

	// Past 30s cooldown, CLI is retried and succeeds.
	mu.Lock()
	curTime = curTime.Add(15 * time.Second)
	shouldFail = false
	mu.Unlock()
	if got := resolver.Resolve(getenv, cliFn); got != "recovered_token" {
		t.Fatalf("got %q after cooldown, want recovered_token", got)
	}
	if calls := cliCalls.Load(); calls != 2 {
		t.Fatalf("cliCalls after cooldown = %d, want 2", calls)
	}
}

func TestAuthResolver_ConcurrentDeduplication(t *testing.T) {
	t.Parallel()

	resolver := NewAuthResolver()
	var cliCalls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	cliFn := func(ctx context.Context) (string, error) {
		if cliCalls.Add(1) == 1 {
			close(started)
			<-release
		}
		return "concurrent_tok", nil
	}

	const workers = 12
	var wg sync.WaitGroup
	results := make([]string, workers)

	wg.Add(1)
	go func() {
		defer wg.Done()
		results[0] = resolver.Resolve(func(string) string { return "" }, cliFn)
	}()

	<-started
	for i := 1; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = resolver.Resolve(func(string) string { return "" }, cliFn)
		}(i)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resolver.mu.Lock()
		w := resolver.waiters
		resolver.mu.Unlock()
		if w >= workers-1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()

	if calls := cliCalls.Load(); calls != 1 {
		t.Fatalf("expected 1 deduplicated CLI call, got %d", calls)
	}
	for i, res := range results {
		if res != "concurrent_tok" {
			t.Errorf("worker %d got %q, want concurrent_tok", i, res)
		}
	}
}

func TestResolveAuthToken_EnvPrecedence(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env_github_tok")
	t.Setenv("GH_TOKEN", "env_gh_tok")

	if got := ResolveAuthToken(); got != "env_github_tok" {
		t.Fatalf("ResolveAuthToken() = %q, want env_github_tok", got)
	}

	t.Setenv("GITHUB_TOKEN", "")
	if got := ResolveAuthToken(); got != "env_gh_tok" {
		t.Fatalf("ResolveAuthToken() with empty GITHUB_TOKEN = %q, want env_gh_tok", got)
	}
}

func TestIsGitHubAuthTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		url   string
		extra []string
		want  bool
	}{
		{
			name: "https api.github.com allowed",
			url:  "https://api.github.com/repos/owner/repo",
			want: true,
		},
		{
			name: "https github.com allowed",
			url:  "https://github.com/octocat",
			want: true,
		},
		{
			name: "https uploads.github.com allowed",
			url:  "https://uploads.github.com/repos/o/r/releases",
			want: true,
		},
		{
			name: "case insensitive host with port 443",
			url:  "https://API.GITHUB.COM:443/user",
			want: true,
		},
		{
			name: "http api.github.com rejected",
			url:  "http://api.github.com/user",
			want: false,
		},
		{
			name:  "http api.github.com rejected even if passed in extra",
			url:   "http://api.github.com/user",
			extra: []string{"http://api.github.com"},
			want:  false,
		},
		{
			name: "evil suffix domain rejected",
			url:  "https://api.github.com.evil.com/user",
			want: false,
		},
		{
			name: "evil prefix domain rejected",
			url:  "https://not-github.com/user",
			want: false,
		},
		{
			name: "userinfo smuggling rejected",
			url:  "https://api.github.com@evil.com/user",
			want: false,
		},
		{
			name: "relative path rejected",
			url:  "/repos/owner/repo",
			want: false,
		},
		{
			name: "javascript scheme rejected",
			url:  "javascript:alert(1)",
			want: false,
		},
		{
			name:  "extra full URL matches exact test server host and port",
			url:   "http://127.0.0.1:54321/repos/o/r",
			extra: []string{"http://127.0.0.1:54321"},
			want:  true,
		},
		{
			name:  "extra full URL rejects different port",
			url:   "http://127.0.0.1:9999/repos/o/r",
			extra: []string{"http://127.0.0.1:54321"},
			want:  false,
		},
		{
			name:  "extra bare host matches",
			url:   "https://ghe.internal.corp/api/v3/user",
			extra: []string{"ghe.internal.corp"},
			want:  true,
		},
		{
			name:  "extra bare host:port matches",
			url:   "http://localhost:8080/graphql",
			extra: []string{"localhost:8080"},
			want:  true,
		},
		{
			name:  "cleartext http non-loopback rejected even in extra",
			url:   "http://ghe.internal.corp/api/v3/user",
			extra: []string{"http://ghe.internal.corp", "ghe.internal.corp"},
			want:  false,
		},
		{
			name:  "extra url without port rejects non-default port",
			url:   "https://ghe.internal.corp:9999/api/v3/user",
			extra: []string{"https://ghe.internal.corp"},
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := IsGitHubAuthTarget(tc.url, tc.extra...)
			if got != tc.want {
				t.Fatalf("IsGitHubAuthTarget(%q, %v) = %v, want %v", tc.url, tc.extra, got, tc.want)
			}
		})
	}
}
