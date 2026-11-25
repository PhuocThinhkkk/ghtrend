package ghclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultAPIBaseURL    = "https://api.github.com"
	defaultGraphQLURL    = "https://api.github.com/graphql"
	defaultWebBaseURL    = "https://github.com"
	defaultAcceptHeader  = "application/vnd.github+json"
	defaultUserAgent     = "ghtrend"
	defaultAPIVersion    = "2022-11-28"
	defaultCacheTTL      = 5 * time.Minute
	defaultHTTPTimeout   = 15 * time.Second
	maxAPIResponseBytes  = 16 << 20 // 16 MiB
	maxMemCacheEntries   = 512
	maxRetryAfterSeconds = int64(math.MaxInt64 / int64(time.Second))
)

var (
	// ErrAuthRequired indicates missing or rejected GitHub API authentication.
	ErrAuthRequired = errors.New("github authentication required")
	// ErrRateLimited indicates a GitHub API rate limit has been exceeded.
	ErrRateLimited = errors.New("github api rate limit exceeded")
	// ErrNotFound indicates the requested GitHub resource was not found.
	ErrNotFound = errors.New("github resource not found")
	// ErrForbidden indicates access to the requested GitHub resource was forbidden.
	ErrForbidden = errors.New("github api access forbidden")
)

var (
	defaultClientMu       sync.Mutex
	fallbackAPIHTTPClient = &http.Client{
		Timeout:       defaultHTTPTimeout,
		CheckRedirect: AuthSafeRedirectPolicy(),
	}
	// DefaultClient is the package-level GitHub API client singleton.
	DefaultClient = NewClient()
)

// ResponseCache defines an optional byte cache for GitHub HTTP GET responses.
type ResponseCache interface {
	GetBytes(key string) ([]byte, bool)
	SetBytes(key string, val []byte, ttl time.Duration)
	Invalidate(key string)
	InvalidatePrefix(prefix string)
}

type memCacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// memCache is a lightweight, thread-safe in-memory ResponseCache fallback.
type memCache struct {
	mu      sync.RWMutex
	entries map[string]memCacheEntry
	now     func() time.Time
}

func newMemCache() *memCache {
	return &memCache{
		entries: make(map[string]memCacheEntry),
		now:     time.Now,
	}
}

func (m *memCache) GetBytes(key string) ([]byte, bool) {
	if m == nil || key == "" {
		return nil, false
	}

	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}

	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}

	if !entry.expiresAt.IsZero() && nowFn().After(entry.expiresAt) {
		m.mu.Lock()
		if cur, exists := m.entries[key]; exists && !cur.expiresAt.IsZero() && nowFn().After(cur.expiresAt) {
			delete(m.entries, key)
		}
		m.mu.Unlock()
		return nil, false
	}

	return slices.Clone(entry.data), true
}

func (m *memCache) SetBytes(key string, val []byte, ttl time.Duration) {
	if m == nil || key == "" {
		return
	}

	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if ttl <= 0 {
		delete(m.entries, key)
		return
	}

	if len(m.entries) >= maxMemCacheEntries {
		now := nowFn()
		for k, e := range m.entries {
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				delete(m.entries, k)
			}
		}
		if len(m.entries) >= maxMemCacheEntries {
			for k := range m.entries {
				delete(m.entries, k)
				if len(m.entries) < maxMemCacheEntries {
					break
				}
			}
		}
	}

	m.entries[key] = memCacheEntry{
		data:      slices.Clone(val),
		expiresAt: nowFn().Add(ttl),
	}
}

func (m *memCache) Invalidate(key string) {
	if m == nil || key == "" {
		return
	}
	m.mu.Lock()
	delete(m.entries, key)
	m.mu.Unlock()
}

func (m *memCache) InvalidatePrefix(prefix string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if prefix == "" {
		clear(m.entries)
		return
	}
	for k := range m.entries {
		if strings.HasPrefix(k, prefix) {
			delete(m.entries, k)
		}
	}
}

type acceptSet struct {
	mu     sync.RWMutex
	values map[string]struct{}
}

func newAcceptSet() *acceptSet {
	return &acceptSet{
		values: make(map[string]struct{}),
	}
}

func (a *acceptSet) add(accept string) {
	if a == nil || accept == "" {
		return
	}
	a.mu.RLock()
	_, exists := a.values[accept]
	a.mu.RUnlock()
	if exists {
		return
	}
	a.mu.Lock()
	a.values[accept] = struct{}{}
	a.mu.Unlock()
}

func (a *acceptSet) list() []string {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, 0, len(a.values))
	for k := range a.values {
		out = append(out, k)
	}
	return out
}

// APIError represents an error from the GitHub REST or GraphQL API, or a local
// pre-flight rate-limit or authentication failure.
type APIError struct {
	StatusCode       int
	Resource         string
	Message          string
	DocumentationURL string
	RateLimited      bool
	RetryAfter       time.Duration
	ResetAt          time.Time
	Err              error
}

// Error returns a concise, actionable lowercase error message.
func (e *APIError) Error() string {
	if e == nil {
		return "github api error"
	}

	msg := strings.TrimSpace(strings.TrimSuffix(e.Message, "."))
	lowerMsg := lowerFirst(msg)

	if e.RateLimited || errors.Is(e.Err, ErrRateLimited) || e.StatusCode == http.StatusTooManyRequests {
		resLabel := ""
		if res := strings.TrimSpace(e.Resource); res != "" {
			resLabel = res + " "
		}
		waitHint := formatAPIWaitHint(e.RetryAfter, e.ResetAt)
		base := fmt.Sprintf("github %srate limit exceeded", resLabel)
		if waitHint != "" {
			base += " (" + waitHint + ")"
		}
		if lowerMsg != "" {
			base += ": " + lowerMsg
		}
		return base + "; set GITHUB_TOKEN or run gh auth login"
	}

	if errors.Is(e.Err, ErrAuthRequired) || e.StatusCode == http.StatusUnauthorized {
		base := "github authentication required"
		if e.StatusCode > 0 {
			base += fmt.Sprintf(" (status %d)", e.StatusCode)
		}
		if lowerMsg != "" {
			base += ": " + lowerMsg
		}
		return base + "; set GITHUB_TOKEN, GH_TOKEN, or run gh auth login"
	}

	if errors.Is(e.Err, ErrNotFound) || e.StatusCode == http.StatusNotFound {
		base := "github resource not found (status 404)"
		if lowerMsg != "" {
			return base + ": " + lowerMsg
		}
		return base
	}

	if errors.Is(e.Err, ErrForbidden) || e.StatusCode == http.StatusForbidden {
		base := "github api access forbidden (status 403)"
		if lowerMsg != "" {
			return base + ": " + lowerMsg
		}
		return base
	}

	if e.StatusCode > 0 {
		if lowerMsg != "" {
			return fmt.Sprintf("github api error (status %d): %s", e.StatusCode, lowerMsg)
		}
		return fmt.Sprintf("github api error (status %d)", e.StatusCode)
	}

	if lowerMsg != "" {
		return "github api error: " + lowerMsg
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "github api error"
}

// Unwrap returns the underlying sentinel or wrapped error.
func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is reports whether target matches the APIError's wrapped error or HTTP classification.
func (e *APIError) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	if e.Err != nil && errors.Is(e.Err, target) {
		return true
	}
	switch target {
	case ErrRateLimited:
		return e.RateLimited || e.StatusCode == http.StatusTooManyRequests
	case ErrAuthRequired:
		return e.StatusCode == http.StatusUnauthorized
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden && !e.RateLimited
	default:
		return false
	}
}

func formatAPIWaitHint(retryAfter time.Duration, resetAt time.Time) string {
	if retryAfter > 0 {
		secs := int64(retryAfter.Round(time.Second).Seconds())
		if secs < 1 {
			secs = 1
		}
		return fmt.Sprintf("retry after %ds", secs)
	}
	if !resetAt.IsZero() {
		return "resets in " + formatResetCountdown(resetAt)
	}
	return ""
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) == 1 {
		return strings.ToLower(s)
	}
	if unicode.IsUpper(r[0]) && !unicode.IsUpper(r[1]) {
		r[0] = unicode.ToLower(r[0])
	}
	return string(r)
}

// Client executes authenticated GitHub REST and GraphQL API requests with
// rate-limit tracking, response caching, and structured error mapping.
type Client struct {
	APIBaseURL      string
	GraphQLURL      string
	WebBaseURL      string
	HTTPClient      *http.Client
	TokenFunc       func() string
	Cache           ResponseCache
	DefaultCacheTTL time.Duration

	cacheMu sync.RWMutex
	accepts *acceptSet
}

// NewClient creates a Client configured with default GitHub endpoints, token discovery,
// bounded HTTP timeout, and an in-memory response cache fallback.
func NewClient() *Client {
	return &Client{
		APIBaseURL:      defaultAPIBaseURL,
		GraphQLURL:      defaultGraphQLURL,
		WebBaseURL:      defaultWebBaseURL,
		HTTPClient:      &http.Client{Timeout: defaultHTTPTimeout, CheckRedirect: AuthSafeRedirectPolicy()},
		TokenFunc:       ResolveAuthToken,
		Cache:           newMemCache(),
		DefaultCacheTTL: defaultCacheTTL,
		accepts:         newAcceptSet(),
	}
}

// SetDefaultCache configures the ResponseCache used by DefaultClient.
// Passing nil restores the default lightweight in-memory cache.
func SetDefaultCache(c ResponseCache) {
	defaultClientMu.Lock()
	defer defaultClientMu.Unlock()

	if DefaultClient == nil {
		DefaultClient = NewClient()
	}
	DefaultClient.cacheMu.Lock()
	defer DefaultClient.cacheMu.Unlock()
	if c == nil {
		DefaultClient.Cache = newMemCache()
		return
	}
	DefaultClient.Cache = c
}

func (c *Client) activeCache() ResponseCache {
	if c == nil {
		return nil
	}
	c.cacheMu.RLock()
	cache := c.Cache
	c.cacheMu.RUnlock()
	return cache
}

const cachedEnvelopeMagic = "\x00GHC1\x00"

func encodeCachedResponse(body []byte, link string) []byte {
	if link == "" && !bytes.HasPrefix(body, []byte(cachedEnvelopeMagic)) {
		return body
	}
	buf := make([]byte, 0, len(cachedEnvelopeMagic)+len(link)+1+len(body))
	buf = append(buf, cachedEnvelopeMagic...)
	buf = append(buf, link...)
	buf = append(buf, 0)
	buf = append(buf, body...)
	return buf
}

func decodeCachedResponse(raw []byte) (body []byte, link string) {
	if !bytes.HasPrefix(raw, []byte(cachedEnvelopeMagic)) {
		return raw, ""
	}
	rest := raw[len(cachedEnvelopeMagic):]
	sep := bytes.IndexByte(rest, 0)
	if sep < 0 {
		return raw, ""
	}
	return rest[sep+1:], string(rest[:sep])
}

// DoRESTRaw executes a GitHub REST API request and returns the raw response body and headers.
func (c *Client) DoRESTRaw(
	ctx context.Context,
	method string,
	pathOrURL string,
	body any,
	accept string,
) ([]byte, http.Header, error) {
	if c == nil {
		return nil, nil, errors.New("ghclient: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}

	resolvedURL, parsedURL, err := c.resolveRESTURL(pathOrURL)
	if err != nil {
		return nil, nil, err
	}

	accept = strings.TrimSpace(accept)
	if accept == "" {
		accept = defaultAcceptHeader
	}

	cacheKey := accept + "|" + resolvedURL
	cacheStore := c.activeCache()
	if method == http.MethodGet && cacheStore != nil {
		if cached, ok := cacheStore.GetBytes(cacheKey); ok {
			bodyBytes, link := decodeCachedResponse(cached)
			headers := http.Header{}
			if link != "" {
				headers.Set("Link", link)
			}
			return bodyBytes, headers, nil
		}
	}

	resource := c.rateLimitResource(parsedURL.Path)
	if err := CheckRateLimit(resource); err != nil {
		return nil, nil, err
	}

	bodyReader, isJSONBody, err := encodeRequestBody(body)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, resolvedURL, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("creating github rest request: %w", err)
	}

	c.applyCommonHeaders(req, resolvedURL, accept, isJSONBody)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing github rest request: %w", err)
	}
	defer resp.Body.Close()

	UpdateRateLimitFromHeaders(resp.Header)

	respBody, err := readBoundedBody(resp.Body)
	if err != nil {
		return nil, resp.Header.Clone(), err
	}

	headers := resp.Header.Clone()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parseAPIError(resp.StatusCode, resp.Header, respBody, resource)
		recordAPIErrorCooldown(apiErr)
		return nil, headers, apiErr
	}

	if cacheStore != nil {
		if method == http.MethodGet {
			if ttl := c.effectiveCacheTTL(); ttl > 0 {
				c.accepts.add(accept)
				cacheStore.SetBytes(cacheKey, encodeCachedResponse(respBody, resp.Header.Get("Link")), ttl)
			}
		} else if isWriteMethod(method) {
			c.invalidateWriteCache(cacheStore, parsedURL, accept)
		}
	}

	return respBody, headers, nil
}

// DoREST executes a GitHub REST API request and decodes the response into out when non-nil.
func (c *Client) DoREST(
	ctx context.Context,
	method string,
	pathOrURL string,
	body any,
	accept string,
	out any,
) (http.Header, error) {
	raw, headers, err := c.DoRESTRaw(ctx, method, pathOrURL, body, accept)
	if err != nil {
		return headers, err
	}

	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		switch dst := out.(type) {
		case *[]byte:
			if dst == nil {
				return headers, errors.New("ghclient: nil *[]byte output pointer")
			}
			*dst = slices.Clone(raw)
		case *string:
			if dst == nil {
				return headers, errors.New("ghclient: nil *string output pointer")
			}
			*dst = string(raw)
		default:
			if err := json.Unmarshal(raw, out); err != nil {
				return headers, fmt.Errorf("decoding github api response: %w", err)
			}
		}
	}

	return headers, nil
}

// DoGraphQL executes a GitHub GraphQL API POST request and unwraps the response data field into out.
// Authentication is required; missing tokens or 401 responses return an error matching ErrAuthRequired.
func (c *Client) DoGraphQL(
	ctx context.Context,
	query string,
	variables map[string]any,
	out any,
) error {
	if c == nil {
		return errors.New("ghclient: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("graphql query cannot be empty")
	}

	token := c.resolveToken()
	if token == "" {
		return &APIError{
			StatusCode: http.StatusUnauthorized,
			Resource:   "graphql",
			Message:    "authentication token missing for graphql request",
			Err:        ErrAuthRequired,
		}
	}

	if err := CheckRateLimit("graphql"); err != nil {
		return err
	}

	payload := struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables,omitempty"`
	}{
		Query:     query,
		Variables: variables,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding graphql payload: %w", err)
	}

	gqlURL := c.graphQLURL()
	parsedGQL, err := url.Parse(gqlURL)
	if err != nil {
		return fmt.Errorf("invalid graphql url %q: %w", gqlURL, err)
	}
	gqlScheme := strings.ToLower(parsedGQL.Scheme)
	if (gqlScheme != "http" && gqlScheme != "https") || parsedGQL.Host == "" {
		return fmt.Errorf("invalid graphql url %q", gqlURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating github graphql request: %w", err)
	}

	c.applyCommonHeaders(req, gqlURL, defaultAcceptHeader, true)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("executing github graphql request: %w", err)
	}
	defer resp.Body.Close()

	UpdateRateLimitFromHeaders(resp.Header)

	respBody, err := readBoundedBody(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parseAPIError(resp.StatusCode, resp.Header, respBody, "graphql")
		recordAPIErrorCooldown(apiErr)
		return apiErr
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("decoding github graphql response: %w", err)
	}

	if len(envelope.Errors) > 0 {
		gqlErr := buildGraphQLError(resp.StatusCode, resp.Header, envelope.Errors)
		if apiErr, ok := gqlErr.(*APIError); ok {
			recordAPIErrorCooldown(apiErr)
		}
		return gqlErr
	}

	if out != nil && len(envelope.Data) > 0 && !bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decoding github graphql data: %w", err)
		}
	}

	return nil
}

func buildGraphQLError(
	statusCode int,
	headers http.Header,
	gqlErrors []struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	},
) error {
	parts := make([]string, 0, len(gqlErrors))
	var mappedErr error
	var rateLimited bool

	for _, ge := range gqlErrors {
		msg := strings.TrimSpace(ge.Message)
		errType := strings.ToUpper(strings.TrimSpace(ge.Type))
		switch errType {
		case "RATE_LIMITED":
			rateLimited = true
			mappedErr = ErrRateLimited
		case "NOT_FOUND":
			if mappedErr == nil {
				mappedErr = ErrNotFound
			}
		case "FORBIDDEN":
			if mappedErr == nil {
				mappedErr = ErrForbidden
			}
		}
		if !rateLimited && strings.Contains(strings.ToLower(msg), "rate limit") {
			rateLimited = true
			mappedErr = ErrRateLimited
		}

		if errType != "" && msg != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", errType, msg))
		} else if msg != "" {
			parts = append(parts, msg)
		} else if errType != "" {
			parts = append(parts, errType)
		}
	}

	joined := strings.Join(parts, "; ")
	if joined == "" {
		joined = "graphql query returned errors"
	}

	return &APIError{
		StatusCode:  statusCode,
		Resource:    "graphql",
		Message:     joined,
		RateLimited: rateLimited,
		RetryAfter:  parseRetryAfter(headers.Get("Retry-After")),
		ResetAt:     parseRateLimitReset(headers.Get("X-Ratelimit-Reset")),
		Err:         mappedErr,
	}
}

func (c *Client) apiBaseURL() string {
	if base := strings.TrimSpace(c.APIBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	return defaultAPIBaseURL
}

func (c *Client) graphQLURL() string {
	if u := strings.TrimSpace(c.GraphQLURL); u != "" {
		return u
	}
	return defaultGraphQLURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return fallbackAPIHTTPClient
}

func (c *Client) resolveToken() string {
	if c.TokenFunc == nil {
		return ""
	}
	return strings.TrimSpace(c.TokenFunc())
}

func (c *Client) allowedAuthHosts() []string {
	hosts := make([]string, 0, 3)
	if u := strings.TrimSpace(c.APIBaseURL); u != "" {
		hosts = append(hosts, u)
	}
	if u := strings.TrimSpace(c.GraphQLURL); u != "" {
		hosts = append(hosts, u)
	}
	if u := strings.TrimSpace(c.WebBaseURL); u != "" {
		hosts = append(hosts, u)
	}
	return hosts
}

func (c *Client) effectiveCacheTTL() time.Duration {
	if c.DefaultCacheTTL < 0 {
		return 0
	}
	if c.DefaultCacheTTL == 0 {
		return defaultCacheTTL
	}
	return c.DefaultCacheTTL
}

func (c *Client) resolveRESTURL(pathOrURL string) (string, *url.URL, error) {
	raw := strings.TrimSpace(pathOrURL)
	if raw == "" {
		return "", nil, errors.New("request path or url cannot be empty")
	}
	if strings.HasPrefix(raw, "//") {
		return "", nil, fmt.Errorf("invalid relative request path %q", raw)
	}

	var resolved string
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		scheme := strings.ToLower(u.Scheme)
		if (scheme != "http" && scheme != "https") || u.Host == "" {
			return "", nil, fmt.Errorf("unsupported url scheme %q", u.Scheme)
		}
		resolved = raw
	} else {
		resolved = c.apiBaseURL() + "/" + strings.TrimLeft(raw, "/")
	}

	parsed, err := url.Parse(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("invalid request url %q: %w", resolved, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "http" && scheme != "https") || parsed.Host == "" {
		return "", nil, fmt.Errorf("invalid request url %q", resolved)
	}

	return resolved, parsed, nil
}

func (c *Client) rateLimitResource(reqPath string) string {
	cleanPath := reqPath
	if baseParsed, err := url.Parse(c.apiBaseURL()); err == nil {
		basePath := strings.TrimRight(baseParsed.Path, "/")
		if basePath != "" && strings.HasPrefix(cleanPath, basePath) {
			cleanPath = strings.TrimPrefix(cleanPath, basePath)
		}
	}
	switch {
	case cleanPath == "/graphql":
		return "graphql"
	case cleanPath == "/search/code" || strings.HasPrefix(cleanPath, "/search/code/"):
		if info := GetRateLimitForResource("code_search"); info.HasData {
			return "code_search"
		}
		return "search"
	case cleanPath == "/search" || strings.HasPrefix(cleanPath, "/search/"):
		return "search"
	default:
		return "core"
	}
}

func (c *Client) applyCommonHeaders(req *http.Request, targetURL, accept string, isJSONBody bool) {
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", defaultAPIVersion)
	if isJSONBody && req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token := c.resolveToken(); token != "" && IsGitHubAuthTarget(targetURL, c.allowedAuthHosts()...) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func (c *Client) invalidateWriteCache(cacheStore ResponseCache, parsedURL *url.URL, currentAccept string) {
	if cacheStore == nil || parsedURL == nil {
		return
	}

	basePath := ""
	if baseParsed, err := url.Parse(c.apiBaseURL()); err == nil {
		basePath = strings.TrimRight(baseParsed.Path, "/")
	}

	prefixes := writeInvalidationURLPrefixes(parsedURL, basePath)
	if len(prefixes) == 0 {
		return
	}

	acceptVariants := collectAcceptVariants(currentAccept, c.accepts.list())
	for _, acc := range acceptVariants {
		for _, urlPrefix := range prefixes {
			cacheStore.InvalidatePrefix(acc + "|" + urlPrefix + "?")
			cacheStore.InvalidatePrefix(acc + "|" + urlPrefix + "/")
			cacheStore.Invalidate(acc + "|" + urlPrefix)
		}
	}
}

func collectAcceptVariants(currentAccept string, seen []string) []string {
	unique := make(map[string]struct{}, len(seen)+2)
	unique[defaultAcceptHeader] = struct{}{}
	if currentAccept != "" {
		unique[currentAccept] = struct{}{}
	}
	for _, s := range seen {
		if s != "" {
			unique[s] = struct{}{}
		}
	}

	out := make([]string, 0, len(unique))
	for acc := range unique {
		out = append(out, acc)
	}
	return out
}

func writeInvalidationURLPrefixes(parsedURL *url.URL, apiBasePath string) []string {
	clean := url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   strings.TrimRight(parsedURL.Path, "/"),
	}
	base := clean.String()
	prefixes := []string{base}

	relPath := clean.Path
	prefixApplied := ""
	if apiBasePath != "" && strings.HasPrefix(relPath, apiBasePath) {
		trimmedRel := strings.TrimPrefix(relPath, apiBasePath)
		if trimmedRel == "" || strings.HasPrefix(trimmedRel, "/") {
			relPath = trimmedRel
			prefixApplied = apiBasePath
		}
	}

	trimmed := strings.Trim(relPath, "/")
	if trimmed == "" {
		return prefixes
	}

	segs := strings.Split(trimmed, "/")
	switch {
	case len(segs) > 4 && segs[0] == "repos":
		// E.g. PATCH /repos/o/r/issues/42 also invalidates cached /repos/o/r/issues lists.
		parent := clean
		parent.Path = prefixApplied + "/" + strings.Join(segs[:len(segs)-1], "/")
		prefixes = append(prefixes, parent.String())
	case len(segs) >= 3 && segs[0] == "user" && (segs[1] == "starred" || segs[1] == "following" || segs[1] == "subscriptions"):
		parent := clean
		parent.Path = prefixApplied + "/" + strings.Join(segs[:2], "/")
		prefixes = append(prefixes, parent.String())
	case len(segs) == 2 && segs[0] == "gists":
		parent := clean
		parent.Path = prefixApplied + "/gists"
		prefixes = append(prefixes, parent.String())
	}

	return prefixes
}

func encodeRequestBody(body any) (io.Reader, bool, error) {
	if body == nil {
		return nil, false, nil
	}
	switch v := body.(type) {
	case *bytes.Reader:
		if v == nil {
			return nil, false, nil
		}
		return v, false, nil
	case *strings.Reader:
		if v == nil {
			return nil, false, nil
		}
		return v, false, nil
	case *bytes.Buffer:
		if v == nil {
			return nil, false, nil
		}
		return v, false, nil
	case io.Reader:
		return v, false, nil
	case []byte:
		return bytes.NewReader(v), false, nil
	case string:
		return strings.NewReader(v), false, nil
	default:
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, false, fmt.Errorf("encoding github request body: %w", err)
		}
		return bytes.NewReader(encoded), true, nil
	}
}

func readBoundedBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxAPIResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading github api response: %w", err)
	}
	if len(data) > maxAPIResponseBytes {
		return nil, fmt.Errorf("github api response exceeded max allowed size of %d bytes", maxAPIResponseBytes)
	}
	return data, nil
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func parseAPIError(statusCode int, headers http.Header, body []byte, defaultResource string) *APIError {
	var ghErr struct {
		Message          string `json:"message"`
		DocumentationURL string `json:"documentation_url"`
	}
	if len(bytes.TrimSpace(body)) > 0 {
		_ = json.Unmarshal(body, &ghErr)
	}

	msg := strings.TrimSpace(ghErr.Message)
	if msg == "" {
		rawText := strings.TrimSpace(string(body))
		if rawText != "" && !strings.HasPrefix(rawText, "<") {
			if runes := []rune(rawText); len(runes) > 256 {
				rawText = string(runes[:256])
			}
			msg = rawText
		} else {
			msg = http.StatusText(statusCode)
		}
	}

	resource := strings.ToLower(strings.TrimSpace(headers.Get("X-Ratelimit-Resource")))
	if resource == "" {
		resource = defaultResource
	}

	retryAfter := parseRetryAfter(headers.Get("Retry-After"))
	resetAt := parseRateLimitReset(headers.Get("X-Ratelimit-Reset"))

	var (
		rateLimited bool
		sentinelErr error
	)

	switch statusCode {
	case http.StatusUnauthorized:
		sentinelErr = ErrAuthRequired
	case http.StatusNotFound:
		sentinelErr = ErrNotFound
	case http.StatusTooManyRequests:
		rateLimited = true
		sentinelErr = ErrRateLimited
	case http.StatusForbidden:
		lowerMsg := strings.ToLower(msg)
		remHeader := strings.TrimSpace(headers.Get("X-Ratelimit-Remaining"))
		isRateLimitMsg := strings.Contains(lowerMsg, "rate limit") || strings.Contains(lowerMsg, "abuse detection")
		if remHeader == "0" || retryAfter > 0 || isRateLimitMsg {
			rateLimited = true
			sentinelErr = ErrRateLimited
		} else {
			sentinelErr = ErrForbidden
		}
	}

	return &APIError{
		StatusCode:       statusCode,
		Resource:         resource,
		Message:          msg,
		DocumentationURL: strings.TrimSpace(ghErr.DocumentationURL),
		RateLimited:      rateLimited,
		RetryAfter:       retryAfter,
		ResetAt:          resetAt,
		Err:              sentinelErr,
	}
}

func recordAPIErrorCooldown(apiErr *APIError) {
	if apiErr == nil || !apiErr.RateLimited {
		return
	}
	now := time.Now()
	var until time.Time
	switch {
	case apiErr.RetryAfter > 0:
		until = now.Add(apiErr.RetryAfter)
	case !apiErr.ResetAt.IsZero() && apiErr.ResetAt.After(now):
		until = apiErr.ResetAt
	default:
		return
	}
	RecordRateLimitCooldown(apiErr.Resource, until)
}

func parseRetryAfter(val string) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	secs, err := strconv.ParseInt(val, 10, 64)
	if err == nil || errors.Is(err, strconv.ErrRange) {
		if errors.Is(err, strconv.ErrRange) {
			if strings.HasPrefix(val, "-") {
				return 0
			}
			secs = maxRetryAfterSeconds
		}
		if secs <= 0 {
			return 0
		}
		if secs > maxRetryAfterSeconds {
			secs = maxRetryAfterSeconds
		}
		return time.Duration(secs) * time.Second
	}
	if retryTime, err := http.ParseTime(val); err == nil {
		d := time.Until(retryTime)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func parseRateLimitReset(val string) time.Time {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}
	}
	if unixSecs, err := strconv.ParseInt(val, 10, 64); err == nil && unixSecs > 0 {
		return time.Unix(unixSecs, 0)
	}
	return time.Time{}
}

// PageInfo holds GraphQL cursor pagination state.
type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// ParseNextPageURL extracts the URL with rel="next" from an RFC 8288 Link header.
// It returns an empty string if no valid next link is present.
func ParseNextPageURL(linkHeader string) string {
	if strings.TrimSpace(linkHeader) == "" {
		return ""
	}

	for _, entry := range splitLinkHeaderEntries(linkHeader) {
		entry = strings.TrimSpace(entry)
		if !strings.HasPrefix(entry, "<") {
			continue
		}
		endBracket := strings.IndexByte(entry, '>')
		if endBracket <= 1 {
			continue
		}

		rawURL := strings.TrimSpace(entry[1:endBracket])
		params := entry[endBracket+1:]
		if !hasNextLinkRel(params) {
			continue
		}

		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		scheme := strings.ToLower(parsed.Scheme)
		if (scheme != "http" && scheme != "https") || parsed.Host == "" {
			continue
		}
		return rawURL
	}

	return ""
}

// HasNextPage reports whether the RFC 8288 Link header contains a valid rel="next" link.
func HasNextPage(linkHeader string) bool {
	return ParseNextPageURL(linkHeader) != ""
}

func splitLinkHeaderEntries(header string) []string {
	entries := []string{}
	start := 0
	inAngle := false
	inQuotes := false
	escaped := false

	for i := 0; i < len(header); i++ {
		ch := header[i]
		if escaped {
			escaped = false
			continue
		}
		if inQuotes {
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inQuotes = false
			}
			continue
		}
		switch ch {
		case '<':
			inAngle = true
		case '>':
			inAngle = false
		case '"':
			if !inAngle {
				inQuotes = true
			}
		case ',':
			if !inAngle && !inQuotes {
				entries = append(entries, header[start:i])
				start = i + 1
			}
		}
	}
	if start <= len(header) {
		entries = append(entries, header[start:])
	}
	return entries
}

func hasNextLinkRel(params string) bool {
	for _, param := range splitSemicolonOutsideQuotes(params) {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		eq := strings.IndexByte(param, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(param[:eq])
		if !strings.EqualFold(key, "rel") {
			continue
		}
		val := strings.TrimSpace(param[eq+1:])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		for _, token := range strings.Fields(val) {
			if strings.EqualFold(token, "next") {
				return true
			}
		}
	}
	return false
}

func splitSemicolonOutsideQuotes(s string) []string {
	parts := []string{}
	start := 0
	inQuotes := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inQuotes {
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inQuotes = false
			}
			continue
		}
		if ch == '"' {
			inQuotes = true
			continue
		}
		if ch == ';' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
