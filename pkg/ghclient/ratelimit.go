package ghclient

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimitInfo holds the current state of GitHub API rate limits.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	ResetTime time.Time
	HasData   bool
}

var (
	rateLimitMu      sync.RWMutex
	currentRateLimit RateLimitInfo
)

// UpdateRateLimitFromHeaders extracts GitHub rate limit headers from HTTP responses.
func UpdateRateLimitFromHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	remStr := headers.Get("X-Ratelimit-Remaining")
	limitStr := headers.Get("X-Ratelimit-Limit")
	if remStr == "" || limitStr == "" {
		return
	}

	rem, err1 := strconv.Atoi(remStr)
	lim, err2 := strconv.Atoi(limitStr)
	if err1 != nil || err2 != nil {
		return
	}

	var resetTime time.Time
	resetStr := headers.Get("X-Ratelimit-Reset")
	if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil && resetUnix > 0 {
		resetTime = time.Unix(resetUnix, 0)
	}

	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	currentRateLimit = RateLimitInfo{
		Limit:     lim,
		Remaining: rem,
		ResetTime: resetTime,
		HasData:   true,
	}
}

// GetRateLimitInfo retrieves the latest recorded GitHub API rate limit status.
func GetRateLimitInfo() RateLimitInfo {
	rateLimitMu.RLock()
	defer rateLimitMu.RUnlock()
	return currentRateLimit
}

// SetRateLimitInfo allows tests or callers to inject rate limit state.
func SetRateLimitInfo(info RateLimitInfo) {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	currentRateLimit = info
}

// FormatRateLimitIndicator formats the rate limit status for display in status bars or UI.
func (r RateLimitInfo) FormatRateLimitIndicator() (text string, isWarning bool, isCritical bool) {
	if !r.HasData {
		return "", false, false
	}

	if r.Remaining == 0 {
		countdown := formatResetCountdown(r.ResetTime)
		return fmt.Sprintf("Rate limit: 0 left (resets in %s)", countdown), false, true
	}

	if r.Remaining <= 10 {
		countdown := formatResetCountdown(r.ResetTime)
		return fmt.Sprintf("Rate limit: %d left (resets in %s)", r.Remaining, countdown), true, false
	}

	return fmt.Sprintf("API: %d/%d", r.Remaining, r.Limit), false, false
}

func formatResetCountdown(resetTime time.Time) string {
	if resetTime.IsZero() {
		return "soon"
	}
	diff := time.Until(resetTime)
	if diff <= 0 {
		return "now"
	}
	if diff < time.Minute {
		return fmt.Sprintf("%ds", int(diff.Seconds()))
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(diff.Hours()), int(diff.Minutes())%60)
}
