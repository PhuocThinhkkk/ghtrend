package ghclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClientAndSetDefaultCache(t *testing.T) {
	c := NewClient()
	if c.APIBaseURL != defaultAPIBaseURL {
		t.Errorf("APIBaseURL = %q, want %q", c.APIBaseURL, defaultAPIBaseURL)
	}
	if c.GraphQLURL != defaultGraphQLURL {
		t.Errorf("GraphQLURL = %q, want %q", c.GraphQLURL, defaultGraphQLURL)
	}
	if c.WebBaseURL != defaultWebBaseURL {
		t.Errorf("WebBaseURL = %q, want %q", c.WebBaseURL, defaultWebBaseURL)
	}
	if c.HTTPClient == nil {
		t.Error("expected non-nil HTTPClient")
	}
	if c.TokenFunc == nil {
		t.Error("expected non-nil TokenFunc")
	}
	if c.Cache == nil {
		t.Error("expected non-nil fallback Cache")
	}
	if c.DefaultCacheTTL <= 0 {
		t.Errorf("expected positive DefaultCacheTTL, got %v", c.DefaultCacheTTL)
	}
	if DefaultClient == nil {
		t.Fatal("expected non-nil DefaultClient")
	}

	origCache := DefaultClient.Cache
	t.Cleanup(func() { SetDefaultCache(origCache) })

	spy := newMemCache()
	SetDefaultCache(spy)
	if DefaultClient.Cache != spy {
		t.Fatal("expected SetDefaultCache to update DefaultClient.Cache")
	}

	SetDefaultCache(nil)
	if DefaultClient.Cache == nil {
		t.Fatal("expected SetDefaultCache(nil) to restore fallback cache")
	}
}

func TestMemCache(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	cur := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	mc := &memCache{
		entries: make(map[string]memCacheEntry),
		now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return cur
		},
	}

	// Defensive copy on set & get.
	input := []byte("hello")
	mc.SetBytes("k1", input, 1*time.Minute)
	input[0] = 'X'

	got, ok := mc.GetBytes("k1")
	if !ok || string(got) != "hello" {
		t.Fatalf("GetBytes(k1) = %q, %v; want hello, true", got, ok)
	}
	got[0] = 'Y'
	got2, ok := mc.GetBytes("k1")
	if !ok || string(got2) != "hello" {
		t.Fatalf("mutating returned slice affected cache: got %q", got2)
	}

	// Expiry check.
	mu.Lock()
	cur = cur.Add(2 * time.Minute)
	mu.Unlock()
	if _, ok := mc.GetBytes("k1"); ok {
		t.Fatal("expected expired entry to miss")
	}

	// Prefix invalidation.
	mc.SetBytes("accept|https://api.github.com/repos/a/b", []byte("1"), time.Minute)
	mc.SetBytes("accept|https://api.github.com/repos/a/b?page=2", []byte("2"), time.Minute)
	mc.SetBytes("accept|https://api.github.com/users/octocat", []byte("3"), time.Minute)

	mc.InvalidatePrefix("accept|https://api.github.com/repos/a/b")
	if _, ok := mc.GetBytes("accept|https://api.github.com/repos/a/b"); ok {
		t.Error("expected exact key to be invalidated")
	}
	if _, ok := mc.GetBytes("accept|https://api.github.com/repos/a/b?page=2"); ok {
		t.Error("expected query key to be invalidated")
	}
	if _, ok := mc.GetBytes("accept|https://api.github.com/users/octocat"); !ok {
		t.Error("expected unrelated key to remain cached")
	}

	// Clear all via empty prefix.
	mc.InvalidatePrefix("")
	if _, ok := mc.GetBytes("accept|https://api.github.com/users/octocat"); ok {
		t.Error("expected empty prefix to invalidate all entries")
	}

	// Capacity bound check.
	for i := 0; i < maxMemCacheEntries+20; i++ {
		mc.SetBytes(fmt.Sprintf("k-%d", i), []byte("v"), time.Minute)
	}
	mc.mu.RLock()
	size := len(mc.entries)
	mc.mu.RUnlock()
	if size > maxMemCacheEntries {
		t.Fatalf("memCache exceeded max entries: %d > %d", size, maxMemCacheEntries)
	}
}

func TestAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          *APIError
		wantIs       []error
		wantNotIs    []error
		wantContains []string
	}{
		{
			name: "401 auth required",
			err: &APIError{
				StatusCode: 401,
				Message:    "Bad credentials",
				Err:        ErrAuthRequired,
			},
			wantIs:       []error{ErrAuthRequired},
			wantNotIs:    []error{ErrNotFound, ErrRateLimited},
			wantContains: []string{"github authentication required", "401", "bad credentials", "GITHUB_TOKEN"},
		},
		{
			name: "429 rate limited",
			err: &APIError{
				StatusCode:  429,
				Resource:    "search",
				Message:     "API rate limit exceeded.",
				RateLimited: true,
				RetryAfter:  30 * time.Second,
				Err:         ErrRateLimited,
			},
			wantIs:       []error{ErrRateLimited},
			wantNotIs:    []error{ErrForbidden},
			wantContains: []string{"github search rate limit exceeded", "retry after 30s", "API rate limit exceeded", "GITHUB_TOKEN"},
		},
		{
			name: "404 not found",
			err: &APIError{
				StatusCode: 404,
				Message:    "Not Found",
				Err:        ErrNotFound,
			},
			wantIs:       []error{ErrNotFound},
			wantNotIs:    []error{ErrAuthRequired},
			wantContains: []string{"github resource not found", "404"},
		},
		{
			name: "403 forbidden without rate limit",
			err: &APIError{
				StatusCode: 403,
				Message:    "Resource not accessible by integration",
				Err:        ErrForbidden,
			},
			wantIs:       []error{ErrForbidden},
			wantNotIs:    []error{ErrRateLimited},
			wantContains: []string{"github api access forbidden", "403"},
		},
		{
			name: "422 validation error",
			err: &APIError{
				StatusCode: 422,
				Message:    "Validation Failed",
			},
			wantNotIs:    []error{ErrAuthRequired, ErrRateLimited, ErrNotFound, ErrForbidden},
			wantContains: []string{"github api error (status 422)", "validation Failed"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msg := tc.err.Error()
			if msg == "" || (msg[0] >= 'A' && msg[0] <= 'Z') {
				t.Errorf("expected lowercase error string, got %q", msg)
			}
			for _, sub := range tc.wantContains {
				if !strings.Contains(msg, sub) {
					t.Errorf("Error() = %q, missing %q", msg, sub)
				}
			}
			for _, target := range tc.wantIs {
				if !errors.Is(tc.err, target) {
					t.Errorf("errors.Is(%v) = false, want true", target)
				}
			}
			for _, target := range tc.wantNotIs {
				if errors.Is(tc.err, target) {
					t.Errorf("errors.Is(%v) = true, want false", target)
				}
			}
		})
	}
}

func TestClient_DoREST_HeadersDecodingAndHostGuard(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	type repoPayload struct {
		FullName string `json:"full_name"`
	}

	var hdrMu sync.Mutex
	var seenAuth string
	var seenUA string
	var seenAccept string
	var seenAPIVer string
	var seenContentType string

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdrMu.Lock()
		seenAuth = r.Header.Get("Authorization")
		seenUA = r.Header.Get("User-Agent")
		seenAccept = r.Header.Get("Accept")
		seenAPIVer = r.Header.Get("X-GitHub-Api-Version")
		seenContentType = r.Header.Get("Content-Type")
		hdrMu.Unlock()

		switch r.URL.Path {
		case "/repos/octocat/hello":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"full_name":"octocat/hello"}`))
		case "/repos/octocat/hello/readme":
			_, _ = w.Write([]byte("# Hello README"))
		case "/user/starred/octocat/hello":
			w.WriteHeader(http.StatusNoContent)
		case "/search/code":
			_, _ = w.Write([]byte(`{"full_name":"search-hit"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	var thirdPartyMu sync.Mutex
	var thirdPartyAuth string
	thirdPartyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		thirdPartyMu.Lock()
		thirdPartyAuth = r.Header.Get("Authorization")
		thirdPartyMu.Unlock()
		_, _ = w.Write([]byte(`{"full_name":"external"}`))
	}))
	defer thirdPartyServer.Close()

	client := NewClient()
	client.APIBaseURL = apiServer.URL
	client.HTTPClient = apiServer.Client()
	client.TokenFunc = func() string { return "secret_token_123" }

	// 1. JSON decode + standard headers.
	var out repoPayload
	if _, err := client.DoREST(context.Background(), http.MethodGet, "/repos/octocat/hello", nil, "", &out); err != nil {
		t.Fatalf("DoREST GET failed: %v", err)
	}
	if out.FullName != "octocat/hello" {
		t.Errorf("FullName = %q, want octocat/hello", out.FullName)
	}
	hdrMu.Lock()
	gotAuth, gotUA, gotAccept, gotAPIVer := seenAuth, seenUA, seenAccept, seenAPIVer
	hdrMu.Unlock()
	if gotAuth != "Bearer secret_token_123" {
		t.Errorf("Authorization = %q, want Bearer secret_token_123", gotAuth)
	}
	if gotUA != "ghtrend" {
		t.Errorf("User-Agent = %q, want ghtrend", gotUA)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept = %q, want application/vnd.github+json", gotAccept)
	}
	if gotAPIVer != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q, want 2022-11-28", gotAPIVer)
	}

	// 2. Custom Accept + *string target.
	var readme string
	if _, err := client.DoREST(context.Background(), http.MethodGet, "repos/octocat/hello/readme", nil, "application/vnd.github.raw+json", &readme); err != nil {
		t.Fatalf("DoREST raw string failed: %v", err)
	}
	hdrMu.Lock()
	gotAccept = seenAccept
	hdrMu.Unlock()
	if readme != "# Hello README" || gotAccept != "application/vnd.github.raw+json" {
		t.Errorf("readme=%q accept=%q", readme, gotAccept)
	}

	// 3. Query string containing http:// still treated as relative path.
	var searchOut repoPayload
	if _, err := client.DoREST(context.Background(), http.MethodGet, "/search/code?q=http://example.com", nil, "", &searchOut); err != nil {
		t.Fatalf("DoREST relative path with URL query param failed: %v", err)
	}
	if searchOut.FullName != "search-hit" {
		t.Errorf("searchOut = %+v", searchOut)
	}

	// 4. Write with struct body sets Content-Type and handles 204 No Content.
	if _, err := client.DoREST(context.Background(), http.MethodPut, "/user/starred/octocat/hello", map[string]string{"note": "star"}, "", nil); err != nil {
		t.Fatalf("DoREST PUT 204 failed: %v", err)
	}
	hdrMu.Lock()
	gotCT := seenContentType
	hdrMu.Unlock()
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}

	// 5. Third-party full URL must NOT receive Authorization header.
	var extOut repoPayload
	if _, err := client.DoREST(context.Background(), http.MethodGet, thirdPartyServer.URL+"/data", nil, "", &extOut); err != nil {
		t.Fatalf("DoREST third-party failed: %v", err)
	}
	thirdPartyMu.Lock()
	gotThirdAuth := thirdPartyAuth
	thirdPartyMu.Unlock()
	if gotThirdAuth != "" {
		t.Fatalf("expected NO Authorization header on third-party host, got %q", gotThirdAuth)
	}
}

func TestClient_DoREST_ErrorMapping(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })

	tests := []struct {
		name           string
		statusCode     int
		headers        map[string]string
		body           string
		wantIs         error
		wantRateLimit  bool
		wantMinRetry   time.Duration
		wantMessageSub string
	}{
		{
			name:           "401 unauthorized",
			statusCode:     401,
			body:           `{"message":"Bad credentials","documentation_url":"https://docs.github.com"}`,
			wantIs:         ErrAuthRequired,
			wantMessageSub: "Bad credentials",
		},
		{
			name:           "403 forbidden without rate limit",
			statusCode:     403,
			headers:        map[string]string{"X-Ratelimit-Remaining": "4999", "X-Ratelimit-Limit": "5000"},
			body:           `{"message":"Must have admin rights to Repository."}`,
			wantIs:         ErrForbidden,
			wantMessageSub: "admin rights",
		},
		{
			name:           "403 primary rate limit exhausted",
			statusCode:     403,
			headers:        map[string]string{"X-Ratelimit-Remaining": "0", "X-Ratelimit-Limit": "60", "X-Ratelimit-Resource": "core"},
			body:           `{"message":"API rate limit exceeded for 127.0.0.1."}`,
			wantIs:         ErrRateLimited,
			wantRateLimit:  true,
			wantMessageSub: "rate limit exceeded",
		},
		{
			name:           "403 secondary rate limit with Retry-After",
			statusCode:     403,
			headers:        map[string]string{"Retry-After": "45"},
			body:           `{"message":"You have exceeded a secondary rate limit."}`,
			wantIs:         ErrRateLimited,
			wantRateLimit:  true,
			wantMinRetry:   45 * time.Second,
			wantMessageSub: "secondary rate limit",
		},
		{
			name:           "404 not found",
			statusCode:     404,
			body:           `{"message":"Not Found"}`,
			wantIs:         ErrNotFound,
			wantMessageSub: "Not Found",
		},
		{
			name:           "422 validation failed",
			statusCode:     422,
			body:           `{"message":"Validation Failed"}`,
			wantMessageSub: "Validation Failed",
		},
		{
			name:           "429 too many requests with clamped overflow Retry-After",
			statusCode:     429,
			headers:        map[string]string{"Retry-After": "9999999999999999999"},
			body:           `{"message":"Too many requests"}`,
			wantIs:         ErrRateLimited,
			wantRateLimit:  true,
			wantMinRetry:   time.Hour,
			wantMessageSub: "Too many requests",
		},
		{
			name:           "429 too many requests with HTTP-date Retry-After",
			statusCode:     429,
			headers:        map[string]string{"Retry-After": time.Now().UTC().Add(120 * time.Second).Format(http.TimeFormat)},
			body:           `{"message":"Rate limit hit"}`,
			wantIs:         ErrRateLimited,
			wantRateLimit:  true,
			wantMinRetry:   30 * time.Second,
			wantMessageSub: "Rate limit hit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetRateLimitInfo(RateLimitInfo{})

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewClient()
			c.APIBaseURL = srv.URL
			c.HTTPClient = srv.Client()
			c.TokenFunc = nil

			_, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r", nil, "")
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T (%v)", err, err)
			}
			if apiErr.StatusCode != tc.statusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.statusCode)
			}
			if apiErr.RateLimited != tc.wantRateLimit {
				t.Errorf("RateLimited = %v, want %v", apiErr.RateLimited, tc.wantRateLimit)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err=%v", tc.wantIs, err)
			}
			if tc.wantMinRetry > 0 && apiErr.RetryAfter < tc.wantMinRetry {
				t.Errorf("RetryAfter = %v, want >= %v", apiErr.RetryAfter, tc.wantMinRetry)
			}
			if tc.wantMessageSub != "" && !strings.Contains(apiErr.Message, tc.wantMessageSub) {
				t.Errorf("Message = %q, want substring %q", apiErr.Message, tc.wantMessageSub)
			}
		})
	}
}

func TestClient_DoREST_RateLimitPreFlightIsolation(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	futureReset := strconv.FormatInt(time.Now().Add(15*time.Minute).Unix(), 10)
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.HasPrefix(r.URL.Path, "/search/") {
			w.Header().Set("X-Ratelimit-Resource", "search")
			w.Header().Set("X-Ratelimit-Limit", "30")
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", futureReset)
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		w.Header().Set("X-Ratelimit-Resource", "core")
		w.Header().Set("X-Ratelimit-Limit", "5000")
		w.Header().Set("X-Ratelimit-Remaining", "4999")
		w.Header().Set("X-Ratelimit-Reset", futureReset)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.APIBaseURL = srv.URL
	c.HTTPClient = srv.Client()
	c.Cache = nil // bypass cache so every allowed call hits server
	c.TokenFunc = nil

	// 1. Search request exhausts "search" resource.
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/search/repositories?q=go", nil, ""); err != nil {
		t.Fatalf("first search failed: %v", err)
	}
	// 2. Second search request is blocked pre-flight without hitting server.
	_, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/search/repositories?q=rust", nil, "")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited for exhausted search, got %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected pre-flight check to block network call; hits = %d", hits.Load())
	}

	// 3. Core request still succeeds while search is exhausted.
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r", nil, ""); err != nil {
		t.Fatalf("core request should not be blocked by search exhaustion: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2", hits.Load())
	}
}

func TestClient_DoREST_CachingAndWriteInvalidation(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	var getHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			n := getHits.Add(1)
			accept := r.Header.Get("Accept")
			_, _ = fmt.Fprintf(w, `{"hit":%d,"accept":%q}`, n, accept)
		case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"updated":true}`))
		}
	}))
	defer srv.Close()

	c := NewClient()
	c.APIBaseURL = srv.URL
	c.HTTPClient = srv.Client()
	c.TokenFunc = nil

	// 1. First GET hits server.
	body1, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?state=open", nil, "")
	if err != nil {
		t.Fatalf("GET 1 failed: %v", err)
	}
	// 2. Second GET hits cache.
	body2, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?state=open", nil, "")
	if err != nil {
		t.Fatalf("GET 2 failed: %v", err)
	}
	if string(body1) != string(body2) || getHits.Load() != 1 {
		t.Fatalf("expected cache hit; hits=%d body1=%s body2=%s", getHits.Load(), body1, body2)
	}

	// 3. Different Accept header uses separate cache key (accept + "|" + resolvedURL).
	bodyRaw, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?state=open", nil, "application/vnd.github.raw+json")
	if err != nil {
		t.Fatalf("GET with custom accept failed: %v", err)
	}
	if getHits.Load() != 2 || !strings.Contains(string(bodyRaw), "raw+json") {
		t.Fatalf("expected distinct cache entry for custom Accept; hits=%d body=%s", getHits.Load(), bodyRaw)
	}

	// 4. PATCH on item invalidates both item and parent collection across Accept variants.
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodPatch, "/repos/o/r/issues/42", map[string]string{"state": "closed"}, ""); err != nil {
		t.Fatalf("PATCH failed: %v", err)
	}

	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?state=open", nil, ""); err != nil {
		t.Fatalf("GET after PATCH failed: %v", err)
	}
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?state=open", nil, "application/vnd.github.raw+json"); err != nil {
		t.Fatalf("GET raw after PATCH failed: %v", err)
	}
	if getHits.Load() != 4 {
		t.Fatalf("expected cache invalidation after PATCH; getHits = %d, want 4", getHits.Load())
	}

	// 5. PUT /user/starred/o/r invalidates /user/starred cache.
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/user/starred", nil, ""); err != nil {
		t.Fatalf("GET /user/starred failed: %v", err)
	}
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodPut, "/user/starred/o/r", nil, ""); err != nil {
		t.Fatalf("PUT /user/starred/o/r failed: %v", err)
	}
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/user/starred", nil, ""); err != nil {
		t.Fatalf("GET /user/starred after PUT failed: %v", err)
	}
	if getHits.Load() != 6 {
		t.Fatalf("expected /user/starred cache invalidation after PUT; getHits = %d, want 6", getHits.Load())
	}
}

func TestClient_DoGraphQL(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })

	t.Run("missing token returns ErrAuthRequired without network", func(t *testing.T) {
		SetRateLimitInfo(RateLimitInfo{})
		var called atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called.Store(true)
		}))
		defer srv.Close()

		c := NewClient()
		c.GraphQLURL = srv.URL
		c.HTTPClient = srv.Client()
		c.TokenFunc = func() string { return "" }

		err := c.DoGraphQL(context.Background(), "query { viewer { login } }", nil, nil)
		if !errors.Is(err, ErrAuthRequired) {
			t.Fatalf("expected ErrAuthRequired, got %v", err)
		}
		if called.Load() {
			t.Fatal("expected no HTTP request when token missing")
		}
	})

	t.Run("success unwraps data and PageInfo", func(t *testing.T) {
		SetRateLimitInfo(RateLimitInfo{})
		var gqlMu sync.Mutex
		var gotQuery string
		var gotLoginVar string
		var gotAuth string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			loginVal, _ := reqBody.Variables["login"].(string)

			gqlMu.Lock()
			gotAuth = r.Header.Get("Authorization")
			gotQuery = reqBody.Query
			if loginVal != "" {
				gotLoginVar = loginVal
			}
			gqlMu.Unlock()

			w.Header().Set("X-Ratelimit-Resource", "graphql")
			w.Header().Set("X-Ratelimit-Limit", "5000")
			w.Header().Set("X-Ratelimit-Remaining", "4998")
			_, _ = w.Write([]byte(`{
				"data": {
					"user": {
						"repositories": {
							"pageInfo": {
								"hasNextPage": true,
								"endCursor": "cursor_abc"
							}
						}
					}
				}
			}`))
		}))
		defer srv.Close()

		c := NewClient()
		c.GraphQLURL = srv.URL
		c.HTTPClient = srv.Client()
		c.TokenFunc = func() string { return "gql_tok" }

		var out struct {
			User struct {
				Repositories struct {
					PageInfo PageInfo `json:"pageInfo"`
				} `json:"repositories"`
			} `json:"user"`
		}
		err := c.DoGraphQL(
			context.Background(),
			"query($login: String!) { user(login: $login) { repositories { pageInfo { hasNextPage endCursor } } } }",
			map[string]any{"login": "octocat"},
			&out,
		)
		if err != nil {
			t.Fatalf("DoGraphQL failed: %v", err)
		}
		gqlMu.Lock()
		authSnap, querySnap, loginSnap := gotAuth, gotQuery, gotLoginVar
		gqlMu.Unlock()
		if authSnap != "Bearer gql_tok" {
			t.Errorf("Authorization = %q, want Bearer gql_tok", authSnap)
		}
		if loginSnap != "octocat" || !strings.Contains(querySnap, "repositories") {
			t.Errorf("unexpected payload query=%q login=%q", querySnap, loginSnap)
		}
		if !out.User.Repositories.PageInfo.HasNextPage || out.User.Repositories.PageInfo.EndCursor != "cursor_abc" {
			t.Errorf("unexpected PageInfo: %+v", out.User.Repositories.PageInfo)
		}
	})

	t.Run("graphql errors and 401 mapping", func(t *testing.T) {
		tests := []struct {
			name       string
			statusCode int
			respBody   string
			wantIs     error
			wantSubstr string
		}{
			{
				name:       "http 401 maps to ErrAuthRequired",
				statusCode: 401,
				respBody:   `{"message":"Bad credentials"}`,
				wantIs:     ErrAuthRequired,
				wantSubstr: "bad credentials",
			},
			{
				name:       "graphql errors array unwraps message",
				statusCode: 200,
				respBody:   `{"data":null,"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a User"}]}`,
				wantIs:     ErrNotFound,
				wantSubstr: "Could not resolve to a User",
			},
			{
				name:       "graphql RATE_LIMITED maps to ErrRateLimited",
				statusCode: 200,
				respBody:   `{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`,
				wantIs:     ErrRateLimited,
				wantSubstr: "API rate limit exceeded",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				SetRateLimitInfo(RateLimitInfo{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte(tc.respBody))
				}))
				defer srv.Close()

				c := NewClient()
				c.GraphQLURL = srv.URL
				c.HTTPClient = srv.Client()
				c.TokenFunc = func() string { return "tok" }

				var out map[string]any
				err := c.DoGraphQL(context.Background(), "query { viewer { login } }", nil, &out)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
					t.Errorf("errors.Is(err, %v) = false; err=%v", tc.wantIs, err)
				}
				if !strings.Contains(err.Error(), tc.wantSubstr) {
					t.Errorf("err.Error() = %q, want %q", err.Error(), tc.wantSubstr)
				}
			})
		}
	})
}

func TestParseNextPageURL_And_HasNextPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		wantURL  string
		wantNext bool
	}{
		{
			name:     "empty header",
			header:   "",
			wantURL:  "",
			wantNext: false,
		},
		{
			name:     "standard github prev next last",
			header:   `<https://api.github.com/user/repos?page=1>; rel="prev", <https://api.github.com/user/repos?page=3>; rel="next", <https://api.github.com/user/repos?page=10>; rel="last"`,
			wantURL:  "https://api.github.com/user/repos?page=3",
			wantNext: true,
		},
		{
			name:     "comma inside query string preserved",
			header:   `<https://api.github.com/search/issues?q=label:bug,help&page=2>; rel="next", <https://api.github.com/search/issues?q=label:bug,help&page=5>; rel="last"`,
			wantURL:  "https://api.github.com/search/issues?q=label:bug,help&page=2",
			wantNext: true,
		},
		{
			name:     "comma inside quoted attribute preserved",
			header:   `<https://api.github.com/items?page=2>; title="next, page"; rel="next prefetch"`,
			wantURL:  "https://api.github.com/items?page=2",
			wantNext: true,
		},
		{
			name:     "unquoted case-insensitive rel",
			header:   `<https://api.github.com/items?page=2>; rel=NEXT`,
			wantURL:  "https://api.github.com/items?page=2",
			wantNext: true,
		},
		{
			name:     "only prev and first",
			header:   `<https://api.github.com/items?page=1>; rel="prev", <https://api.github.com/items?page=1>; rel="first"`,
			wantURL:  "",
			wantNext: false,
		},
		{
			name:     "rel next-page does not false match",
			header:   `<https://api.github.com/items?page=2>; rel="next-page"`,
			wantURL:  "",
			wantNext: false,
		},
		{
			name:     "non-http scheme rejected",
			header:   `<javascript:alert(1)>; rel="next"`,
			wantURL:  "",
			wantNext: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ParseNextPageURL(tc.header); got != tc.wantURL {
				t.Errorf("ParseNextPageURL() = %q, want %q", got, tc.wantURL)
			}
			if got := HasNextPage(tc.header); got != tc.wantNext {
				t.Errorf("HasNextPage() = %v, want %v", got, tc.wantNext)
			}
		})
	}
}

func TestClient_DoREST_CachedLinkHeaderAndRedirectAuthStrip(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	var obsMu sync.Mutex
	var extObserved bool
	var extAuth string
	extSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		obsMu.Lock()
		extObserved = true
		extAuth = r.Header.Get("Authorization")
		obsMu.Unlock()
		_, _ = w.Write([]byte(`{"redirected":true}`))
	}))
	defer extSrv.Close()

	var hits atomic.Int32
	var redirectStartAuth string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/issues":
			hits.Add(1)
			w.Header().Add("Link", `<https://api.github.com/repos/o/r/issues?page=1>; rel="prev"`)
			w.Header().Add("Link", `<https://api.github.com/repos/o/r/issues?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[{"number":1}]`))
		case "/redirect":
			obsMu.Lock()
			redirectStartAuth = r.Header.Get("Authorization")
			obsMu.Unlock()
			http.Redirect(w, r, extSrv.URL+"/target", http.StatusFound)
		case "/same-origin-redirect":
			http.Redirect(w, r, "/repos/o/r/issues?page=1", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	c := NewClient()
	c.APIBaseURL = apiSrv.URL
	c.TokenFunc = func() string { return "secret_tok" }
	c.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	// 1. Multi-Link headers must be preserved across cache hit.
	_, hdr1, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?page=1", nil, "")
	if err != nil {
		t.Fatalf("first GET failed: %v", err)
	}
	_, hdr2, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues?page=1", nil, "")
	if err != nil {
		t.Fatalf("second GET failed: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 network hit, got %d", hits.Load())
	}
	if got := NextPageURL(hdr1, apiSrv.URL); got != "https://api.github.com/repos/o/r/issues?page=2" {
		t.Fatalf("hdr1 NextPageURL = %q", got)
	}
	if got := NextPageURL(hdr2, apiSrv.URL); got != "https://api.github.com/repos/o/r/issues?page=2" {
		t.Fatalf("cached hdr2 NextPageURL = %q, want preserved next URL", got)
	}

	// 2. Typed-nil pointers return error rather than panicking.
	var nilBytes *[]byte
	if _, err := c.DoREST(context.Background(), http.MethodGet, "/repos/o/r/issues?page=1", nil, "", nilBytes); err == nil {
		t.Fatal("expected error for typed-nil *[]byte target")
	}

	// 3. Cross-host redirect starts with Authorization and strips it at untrusted destination.
	var out map[string]any
	if _, err := c.DoREST(context.Background(), http.MethodGet, "/redirect", nil, "", &out); err != nil {
		t.Fatalf("redirect request failed: %v", err)
	}
	obsMu.Lock()
	startAuthSnap, extObsSnap, extAuthSnap := redirectStartAuth, extObserved, extAuth
	obsMu.Unlock()
	if startAuthSnap != "Bearer secret_tok" {
		t.Fatalf("expected /redirect to receive initial Bearer secret_tok, got %q", startAuthSnap)
	}
	if !extObsSnap || extAuthSnap != "" {
		t.Fatalf("expected external destination observed=%v with empty Authorization=%q", extObsSnap, extAuthSnap)
	}
}

func TestClient_DoREST_CredentialAndCanonicalIsolation(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		_, _ = fmt.Fprintf(w, `{"hit":%d,"auth":%q}`, n, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	tok := "token_alpha"
	var tokMu sync.Mutex
	c := NewClient()
	c.APIBaseURL = srv.URL
	c.HTTPClient = srv.Client()
	c.TokenFunc = func() string {
		tokMu.Lock()
		defer tokMu.Unlock()
		return tok
	}

	// 1. Credential change misses old credential cache entry.
	b1, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues/42", nil, "")
	if err != nil {
		t.Fatalf("GET token_alpha failed: %v", err)
	}
	tokMu.Lock()
	tok = "token_beta"
	tokMu.Unlock()
	b2, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues/42", nil, "")
	if err != nil {
		t.Fatalf("GET token_beta failed: %v", err)
	}
	if string(b1) == string(b2) || hits.Load() != 2 {
		t.Fatalf("expected distinct cache keys for different credentials; hits=%d b1=%s b2=%s", hits.Load(), b1, b2)
	}

	// 2. Canonical URL matching: %34%32 hits same cache & gets invalidated by PATCH /42.
	_, _, err = c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues/%34%32", nil, "")
	if err != nil {
		t.Fatalf("GET %%34%%32 failed: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expected canonical cache hit for %%34%%32; hits=%d", hits.Load())
	}
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodPatch, "/repos/o/r/issues/42", map[string]string{"state": "closed"}, ""); err != nil {
		t.Fatalf("PATCH /42 failed: %v", err)
	}
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r/issues/%34%32", nil, ""); err != nil {
		t.Fatalf("GET %%34%%32 after PATCH failed: %v", err)
	}
	if hits.Load() != 4 {
		t.Fatalf("expected PATCH /42 to invalidate %%34%%32 cache; hits=%d", hits.Load())
	}

	// 3. Relative next link resolution.
	if got := ParseNextPageURLWithBase(`<?page=3>; rel="next"`, srv.URL+"/repos/o/r/issues?page=2"); got != srv.URL+"/repos/o/r/issues?page=3" {
		t.Fatalf("ParseNextPageURLWithBase relative = %q", got)
	}
	if !HasNextPage(`<?page=2>; rel="next"`) {
		t.Fatal("expected HasNextPage to accept valid relative link reference")
	}
}

func TestClient_DoREST_SecondaryRateLimitCooldown(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "45")
		w.Header().Set("X-Ratelimit-Resource", "core")
		w.Header().Set("X-Ratelimit-Remaining", "4990")
		w.Header().Set("X-Ratelimit-Limit", "5000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit."}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.APIBaseURL = srv.URL
	c.HTTPClient = srv.Client()
	c.TokenFunc = nil

	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r", nil, ""); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited on secondary rate limit, got %v", err)
	}
	if _, _, err := c.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r", nil, ""); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected pre-flight ErrRateLimited during cooldown, got %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected cooldown to prevent second network hit; hits=%d", hits.Load())
	}

	// Isolated client with a different credential must not be blocked by c's cooldown.
	c2 := NewClient()
	c2.APIBaseURL = srv.URL
	c2.HTTPClient = srv.Client()
	c2.TokenFunc = func() string { return "other_user_tok" }
	_, _, _ = c2.DoRESTRaw(context.Background(), http.MethodGet, "/repos/o/r", nil, "")
	if hits.Load() != 2 {
		t.Fatalf("expected isolated credential client to hit server; hits=%d", hits.Load())
	}
}

func TestSetDefaultCache_ConcurrentWithRequests(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	origCache := DefaultClient.activeCache()
	origURL := DefaultClient.APIBaseURL
	origToken := DefaultClient.TokenFunc
	t.Cleanup(func() {
		SetDefaultCache(origCache)
		DefaultClient.APIBaseURL = origURL
		DefaultClient.TokenFunc = origToken
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	DefaultClient.APIBaseURL = srv.URL
	DefaultClient.TokenFunc = func() string { return "" }

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				SetDefaultCache(newMemCache())
			} else {
				_, _, errs[idx] = DefaultClient.DoRESTRaw(context.Background(), http.MethodGet, "/ping", nil, "")
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d failed: %v", i, err)
		}
	}
}
