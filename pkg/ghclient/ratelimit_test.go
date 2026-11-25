package ghclient

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestUpdateRateLimitFromHeaders(t *testing.T) {
	SetRateLimitInfo(RateLimitInfo{})
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })

	resetEpoch := time.Now().Add(15 * time.Minute).Unix()

	headers := http.Header{}
	headers.Set("X-Ratelimit-Limit", "5000")
	headers.Set("X-Ratelimit-Remaining", "4980")
	headers.Set("X-Ratelimit-Reset", "1700000000")

	UpdateRateLimitFromHeaders(headers)
	info := GetRateLimitInfo()

	if !info.HasData {
		t.Fatal("expected HasData to be true")
	}
	if info.Limit != 5000 {
		t.Errorf("expected limit 5000, got %d", info.Limit)
	}
	if info.Remaining != 4980 {
		t.Errorf("expected remaining 4980, got %d", info.Remaining)
	}

	text, isWarn, isCrit := info.FormatRateLimitIndicator()
	if text != "API: 4980/5000" {
		t.Errorf("expected 'API: 4980/5000', got %q", text)
	}
	if isWarn || isCrit {
		t.Errorf("expected normal styling, got warn=%v crit=%v", isWarn, isCrit)
	}

	// Test warning threshold (<= 10)
	SetRateLimitInfo(RateLimitInfo{
		Limit:     60,
		Remaining: 5,
		ResetTime: time.Unix(resetEpoch, 0),
		HasData:   true,
	})
	info = GetRateLimitInfo()
	text, isWarn, isCrit = info.FormatRateLimitIndicator()
	if !isWarn || isCrit {
		t.Errorf("expected warning styling for remaining 5, got warn=%v crit=%v", isWarn, isCrit)
	}

	// Test critical threshold (0)
	SetRateLimitInfo(RateLimitInfo{
		Limit:     60,
		Remaining: 0,
		ResetTime: time.Unix(resetEpoch, 0),
		HasData:   true,
	})
	info = GetRateLimitInfo()
	text, isWarn, isCrit = info.FormatRateLimitIndicator()
	if !isCrit {
		t.Errorf("expected critical styling for remaining 0, got text=%q crit=%v", text, isCrit)
	}
}

func TestUpdateRateLimitFromHeaders_ResourcesAndPrecedence(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })

	futureReset := strconv.FormatInt(time.Now().Add(20*time.Minute).Unix(), 10)

	tests := []struct {
		name            string
		steps           []http.Header
		wantDefaultLim  int
		wantDefaultRem  int
		wantDefaultRes  string
		wantResource    string
		wantResourceRem int
		wantResourceHas bool
	}{
		{
			name: "graphql populates default when core unset",
			steps: []http.Header{
				makeRateHeaders("graphql", "5000", "4995", futureReset),
			},
			wantDefaultLim:  5000,
			wantDefaultRem:  4995,
			wantDefaultRes:  "graphql",
			wantResource:    "graphql",
			wantResourceRem: 4995,
			wantResourceHas: true,
		},
		{
			name: "search does not overwrite default or core",
			steps: []http.Header{
				makeRateHeaders("core", "5000", "4950", futureReset),
				makeRateHeaders("search", "30", "28", futureReset),
			},
			wantDefaultLim:  5000,
			wantDefaultRem:  4950,
			wantDefaultRes:  "core",
			wantResource:    "search",
			wantResourceRem: 28,
			wantResourceHas: true,
		},
		{
			name: "core replaces prior graphql fallback default",
			steps: []http.Header{
				makeRateHeaders("graphql", "5000", "4999", futureReset),
				makeRateHeaders("core", "5000", "4500", futureReset),
			},
			wantDefaultLim:  5000,
			wantDefaultRem:  4500,
			wantDefaultRes:  "core",
			wantResource:    "graphql",
			wantResourceRem: 4999,
			wantResourceHas: true,
		},
		{
			name: "graphql does not overwrite populated core",
			steps: []http.Header{
				makeRateHeaders("core", "5000", "4100", futureReset),
				makeRateHeaders("graphql", "5000", "4990", futureReset),
			},
			wantDefaultLim:  5000,
			wantDefaultRem:  4100,
			wantDefaultRes:  "core",
			wantResource:    "graphql",
			wantResourceRem: 4990,
			wantResourceHas: true,
		},
		{
			name: "invalid headers ignored",
			steps: []http.Header{
				makeRateHeaders("core", "5000", "4200", futureReset),
				makeRateHeaders("core", "not-an-int", "10", futureReset),
				makeRateHeaders("core", "5000", "-1", futureReset),
				nil,
			},
			wantDefaultLim:  5000,
			wantDefaultRem:  4200,
			wantDefaultRes:  "core",
			wantResource:    "core",
			wantResourceRem: 4200,
			wantResourceHas: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetRateLimitInfo(RateLimitInfo{})
			for _, h := range tc.steps {
				UpdateRateLimitFromHeaders(h)
			}

			def := GetRateLimitInfo()
			if def.Limit != tc.wantDefaultLim || def.Remaining != tc.wantDefaultRem || def.Resource != tc.wantDefaultRes {
				t.Fatalf("default = %+v, want limit=%d rem=%d res=%q", def, tc.wantDefaultLim, tc.wantDefaultRem, tc.wantDefaultRes)
			}

			gotRes := GetRateLimitForResource(tc.wantResource)
			if gotRes.HasData != tc.wantResourceHas || gotRes.Remaining != tc.wantResourceRem {
				t.Fatalf("resource %q = %+v, want has=%v rem=%d", tc.wantResource, gotRes, tc.wantResourceHas, tc.wantResourceRem)
			}
		})
	}
}

func TestUpdateRateLimitFromHeaders_OutOfOrderWindowGuard(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })

	reset1 := strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10)
	reset2 := strconv.FormatInt(time.Now().Add(70*time.Minute).Unix(), 10)

	SetRateLimitInfo(RateLimitInfo{})
	UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", "4900", reset1))
	// Out-of-order response from same reset window with higher remaining must NOT regress upward.
	UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", "4950", reset1))

	if got := GetRateLimitInfo(); got.Remaining != 4900 {
		t.Fatalf("expected remaining to stay 4900, got %d", got.Remaining)
	}
	if got := GetRateLimitForResource("core"); got.Remaining != 4900 {
		t.Fatalf("expected core resource remaining to stay 4900, got %d", got.Remaining)
	}

	// Lower remaining in the same window updates normally.
	UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", "4890", reset1))
	if got := GetRateLimitInfo(); got.Remaining != 4890 {
		t.Fatalf("expected remaining 4890, got %d", got.Remaining)
	}

	// New reset window allows higher remaining.
	UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", "5000", reset2))
	if got := GetRateLimitInfo(); got.Remaining != 5000 {
		t.Fatalf("expected remaining 5000 in new reset window, got %d", got.Remaining)
	}
}

func TestCheckRateLimit(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })

	futureReset := time.Now().Add(10 * time.Minute)
	pastReset := time.Now().Add(-1 * time.Minute)

	tests := []struct {
		name      string
		setup     func()
		resource  string
		wantErr   bool
		wantIs    error
		wantRetry bool
	}{
		{
			name:     "no data allows request",
			setup:    func() { SetRateLimitInfo(RateLimitInfo{}) },
			resource: "core",
			wantErr:  false,
		},
		{
			name: "remaining > 0 allows request",
			setup: func() {
				SetRateLimitInfo(RateLimitInfo{Limit: 60, Remaining: 1, ResetTime: futureReset, HasData: true})
			},
			resource: "core",
			wantErr:  false,
		},
		{
			name: "exhausted core with future reset returns ErrRateLimited",
			setup: func() {
				SetRateLimitInfo(RateLimitInfo{Limit: 60, Remaining: 0, ResetTime: futureReset, HasData: true})
			},
			resource:  "core",
			wantErr:   true,
			wantIs:    ErrRateLimited,
			wantRetry: true,
		},
		{
			name: "exhausted core with zero reset returns ErrRateLimited",
			setup: func() {
				SetRateLimitInfo(RateLimitInfo{Limit: 60, Remaining: 0, HasData: true})
			},
			resource: "core",
			wantErr:  true,
			wantIs:   ErrRateLimited,
		},
		{
			name: "exhausted core with past reset allows probe request",
			setup: func() {
				SetRateLimitInfo(RateLimitInfo{Limit: 60, Remaining: 0, ResetTime: pastReset, HasData: true})
			},
			resource: "core",
			wantErr:  false,
		},
		{
			name: "exhausted search does not block core",
			setup: func() {
				SetRateLimitInfo(RateLimitInfo{Limit: 5000, Remaining: 4999, ResetTime: futureReset, HasData: true})
				UpdateRateLimitFromHeaders(makeRateHeaders("search", "30", "0", strconv.FormatInt(futureReset.Unix(), 10)))
			},
			resource: "core",
			wantErr:  false,
		},
		{
			name: "exhausted search blocks search",
			setup: func() {
				SetRateLimitInfo(RateLimitInfo{Limit: 5000, Remaining: 4999, ResetTime: futureReset, HasData: true})
				UpdateRateLimitFromHeaders(makeRateHeaders("search", "30", "0", strconv.FormatInt(futureReset.Unix(), 10)))
			},
			resource:  "search",
			wantErr:   true,
			wantIs:    ErrRateLimited,
			wantRetry: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			err := CheckRateLimit(tc.resource)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CheckRateLimit(%q) err = %v, wantErr = %v", tc.resource, err, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(err, tc.wantIs) {
					t.Errorf("errors.Is(err, %v) = false; err = %v", tc.wantIs, err)
				}
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T", err)
				}
				if apiErr.Resource != tc.resource {
					t.Errorf("Resource = %q, want %q", apiErr.Resource, tc.resource)
				}
				if tc.wantRetry && apiErr.RetryAfter <= 0 {
					t.Errorf("expected positive RetryAfter, got %v", apiErr.RetryAfter)
				}
			}
		})
	}
}

func TestUpdateRateLimitFromHeaders_Concurrent(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	resetUnix := strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10)
	var wg sync.WaitGroup
	for rem := 5000; rem >= 4900; rem-- {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", strconv.Itoa(r), resetUnix))
			_ = GetRateLimitInfo()
			_ = GetRateLimitForResource("core")
			_ = CheckRateLimit("core")
		}(rem)
	}
	wg.Wait()

	if got := GetRateLimitForResource("core"); got.Remaining != 4900 {
		t.Fatalf("expected lowest remaining 4900 after concurrent same-window updates, got %d", got.Remaining)
	}
}

func TestUpdateRateLimitFromHeaders_IgnoresOlderResetWindow(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{})

	newerReset := strconv.FormatInt(time.Now().Add(45*time.Minute).Unix(), 10)
	olderReset := strconv.FormatInt(time.Now().Add(-5*time.Minute).Unix(), 10)

	UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", "4995", newerReset))
	UpdateRateLimitFromHeaders(makeRateHeaders("core", "5000", "0", olderReset))

	got := GetRateLimitForResource("core")
	if got.Remaining != 4995 {
		t.Fatalf("expected newer reset window remaining 4995 to be preserved, got %d", got.Remaining)
	}
}

func TestCheckRateLimit_Cooldown(t *testing.T) {
	t.Cleanup(func() { SetRateLimitInfo(RateLimitInfo{}) })
	SetRateLimitInfo(RateLimitInfo{Limit: 5000, Remaining: 4990, ResetTime: time.Now().Add(time.Hour), HasData: true})

	RecordRateLimitCooldown("core", time.Now().Add(30*time.Second))
	err := CheckRateLimit("core")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited during active cooldown, got %v", err)
	}
}

func makeRateHeaders(resource, limit, remaining, reset string) http.Header {
	h := http.Header{}
	if resource != "" {
		h.Set("X-Ratelimit-Resource", resource)
	}
	h.Set("X-Ratelimit-Limit", limit)
	h.Set("X-Ratelimit-Remaining", remaining)
	h.Set("X-Ratelimit-Reset", reset)
	return h
}
