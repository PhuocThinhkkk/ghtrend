package ghclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	defaultCLITokenTimeout = 2 * time.Second
	defaultCLIFailCooldown = 30 * time.Second
)

// AuthResolver resolves GitHub authentication tokens with live environment variable
// precedence and mutex-protected caching for `gh auth token` CLI fallback.
type AuthResolver struct {
	mu           sync.Mutex
	cliToken     string
	hasToken     bool
	lastFailure  time.Time
	failCooldown time.Duration
	now          func() time.Time
	inflight     chan struct{}
}

// NewAuthResolver creates an AuthResolver with a 30-second CLI failure cooldown.
func NewAuthResolver() *AuthResolver {
	return &AuthResolver{
		failCooldown: defaultCLIFailCooldown,
		now:          time.Now,
	}
}

var defaultAuthResolver = NewAuthResolver()

// ResolveAuthToken returns a GitHub authentication token using the discovery order:
// GITHUB_TOKEN -> GH_TOKEN -> cached `gh auth token` CLI output.
// Environment variables are always evaluated live and take precedence over cached CLI tokens.
func ResolveAuthToken() string {
	return defaultAuthResolver.Resolve(os.Getenv, execGHAuthToken)
}

// ResolveAuthTokenWith resolves a GitHub authentication token using the supplied environment
// lookup and CLI token callback without mutating package-global cache state.
func ResolveAuthTokenWith(
	getenv func(string) string,
	cliToken func(context.Context) (string, error),
) string {
	if token := lookupEnvToken(getenv); token != "" {
		return token
	}
	if cliToken == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultCLITokenTimeout)
	defer cancel()

	tok, err := cliToken(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(tok)
}

// Resolve evaluates environment variables live and falls back to the cached CLI callback.
func (r *AuthResolver) Resolve(
	getenv func(string) string,
	cliToken func(context.Context) (string, error),
) string {
	if token := lookupEnvToken(getenv); token != "" {
		return token
	}
	if r == nil {
		return ResolveAuthTokenWith(getenv, cliToken)
	}
	if cliToken == nil {
		return ""
	}

	nowFn := r.now
	if nowFn == nil {
		nowFn = time.Now
	}
	cooldown := r.failCooldown
	if cooldown <= 0 {
		cooldown = defaultCLIFailCooldown
	}

	for {
		r.mu.Lock()
		if r.hasToken {
			tok := r.cliToken
			r.mu.Unlock()
			return tok
		}
		if !r.lastFailure.IsZero() && nowFn().Sub(r.lastFailure) < cooldown {
			r.mu.Unlock()
			return ""
		}
		if r.inflight != nil {
			ch := r.inflight
			r.mu.Unlock()
			<-ch
			continue
		}

		ch := make(chan struct{})
		r.inflight = ch
		r.mu.Unlock()

		var (
			resolved string
			cliErr   error
		)
		func() {
			defer func() {
				r.mu.Lock()
				if r.inflight == ch {
					r.inflight = nil
				}
				if cliErr == nil && resolved != "" {
					r.cliToken = resolved
					r.hasToken = true
					r.lastFailure = time.Time{}
				} else {
					r.lastFailure = nowFn()
				}
				close(ch)
				r.mu.Unlock()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), defaultCLITokenTimeout)
			defer cancel()

			tok, err := cliToken(ctx)
			cliErr = err
			if err == nil {
				resolved = strings.TrimSpace(tok)
			}
		}()

		return resolved
	}
}

func lookupEnvToken(getenv func(string) string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	if token := strings.TrimSpace(getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	if token := strings.TrimSpace(getenv("GH_TOKEN")); token != "" {
		return token
	}
	return ""
}

func execGHAuthToken(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("executing gh auth token: %w", err)
	}
	tok := strings.TrimSpace(string(out))
	if tok == "" {
		return "", errors.New("gh auth token produced empty output")
	}
	return tok, nil
}

// IsGitHubAuthTarget reports whether rawURL is an authorized GitHub or explicitly configured
// host to receive an Authorization header. Non-loopback hosts always require HTTPS.
func IsGitHubAuthTarget(rawURL string, extraAllowedHosts ...string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return false
	}

	targetHostname := strings.ToLower(u.Hostname())
	if targetHostname == "" {
		return false
	}

	if scheme == "http" && !isLoopbackHost(targetHostname) {
		return false
	}

	targetPort := effectivePort(scheme, u.Port())
	if isDefaultGitHubHost(targetHostname) {
		return scheme == "https" && targetPort == "443"
	}

	for _, allowed := range extraAllowedHosts {
		if matchesAllowedHost(scheme, targetHostname, targetPort, allowed) {
			return true
		}
	}

	return false
}

// AuthSafeRedirectPolicy returns an http.Client CheckRedirect policy that strips
// Authorization headers whenever a redirect crosses to a host outside the allowed targets.
func AuthSafeRedirectPolicy(extraAllowedHosts ...string) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if req != nil && req.URL != nil && !IsGitHubAuthTarget(req.URL.String(), extraAllowedHosts...) {
			req.Header.Del("Authorization")
		}
		return nil
	}
}

func isDefaultGitHubHost(hostname string) bool {
	switch hostname {
	case "github.com", "api.github.com", "uploads.github.com":
		return true
	default:
		return false
	}
}

func isLoopbackHost(hostname string) bool {
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(hostname, "[]"))
	return ip != nil && ip.IsLoopback()
}

func effectivePort(scheme, explicitPort string) string {
	if explicitPort != "" {
		return explicitPort
	}
	if strings.EqualFold(scheme, "http") {
		return "80"
	}
	return "443"
}

func matchesAllowedHost(scheme, targetHostname, targetPort, allowed string) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" {
		return false
	}

	if strings.Contains(allowed, "://") {
		au, err := url.Parse(allowed)
		if err != nil || au.Host == "" || au.User != nil {
			return false
		}
		if !strings.EqualFold(scheme, au.Scheme) {
			return false
		}
		allowedHostname := strings.ToLower(au.Hostname())
		if isDefaultGitHubHost(allowedHostname) && scheme != "https" {
			return false
		}
		if scheme == "http" && !isLoopbackHost(allowedHostname) {
			return false
		}
		return strings.EqualFold(targetHostname, allowedHostname) && targetPort == effectivePort(au.Scheme, au.Port())
	}

	if hasExplicitPort(allowed) {
		host, port, err := net.SplitHostPort(allowed)
		if err != nil {
			return false
		}
		allowedHostname := strings.ToLower(strings.Trim(host, "[]"))
		if isDefaultGitHubHost(allowedHostname) && scheme != "https" {
			return false
		}
		if scheme == "http" && !isLoopbackHost(allowedHostname) {
			return false
		}
		return strings.EqualFold(targetHostname, allowedHostname) && targetPort == port
	}

	cleanAllowed := strings.ToLower(strings.Trim(allowed, "[]"))
	if isDefaultGitHubHost(cleanAllowed) && scheme != "https" {
		return false
	}
	if scheme == "http" && !isLoopbackHost(cleanAllowed) {
		return false
	}
	return strings.EqualFold(targetHostname, cleanAllowed)
}

func hasExplicitPort(hostPort string) bool {
	if strings.HasPrefix(hostPort, "[") {
		return strings.Contains(hostPort, "]:")
	}
	return strings.Count(hostPort, ":") == 1
}
