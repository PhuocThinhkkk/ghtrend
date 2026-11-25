package ghclient

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitInfo holds the current state of GitHub API rate limits.
type RateLimitInfo struct {
	Resource  string
	Limit     int
	Remaining int
	ResetTime time.Time
	HasData   bool
}

var (
	rateLimitMu       sync.RWMutex
	currentRateLimit  RateLimitInfo
	resourceLimits    = map[string]RateLimitInfo{}
	resourceCooldowns = map[string]time.Time{}
)

// UpdateRateLimitFromHeaders extracts GitHub rate limit headers from HTTP responses.
func UpdateRateLimitFromHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	remStr := strings.TrimSpace(headers.Get("X-Ratelimit-Remaining"))
	limitStr := strings.TrimSpace(headers.Get("X-Ratelimit-Limit"))
	if remStr == "" || limitStr == "" {
		return
	}

	rem, err1 := strconv.Atoi(remStr)
	lim, err2 := strconv.Atoi(limitStr)
	if err1 != nil || err2 != nil || rem < 0 || lim < 0 {
		return
	}

	var resetTime time.Time
	resetStr := strings.TrimSpace(headers.Get("X-Ratelimit-Reset"))
	if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil && resetUnix > 0 {
		resetTime = time.Unix(resetUnix, 0)
	}

	resource := strings.ToLower(strings.TrimSpace(headers.Get("X-Ratelimit-Resource")))
	incoming := RateLimitInfo{
		Resource:  resource,
		Limit:     lim,
		Remaining: rem,
		ResetTime: resetTime,
		HasData:   true,
	}

	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()

	switch resource {
	case "":
		// Legacy or mock headers without X-Ratelimit-Resource update both default and core state.
		if !shouldSkipStaleUpdate(currentRateLimit, incoming) {
			currentRateLimit = incoming
		}
		coreIncoming := incoming
		coreIncoming.Resource = "core"
		if !shouldSkipStaleUpdate(resourceLimits["core"], coreIncoming) {
			resourceLimits["core"] = coreIncoming
		}
	case "core":
		if !shouldSkipStaleUpdate(resourceLimits["core"], incoming) {
			resourceLimits["core"] = incoming
		}
		// Replace non-core fallback (such as graphql) immediately, otherwise guard same-window order.
		isCoreOrLegacy := currentRateLimit.Resource == "" || currentRateLimit.Resource == "core"
		if !isCoreOrLegacy || !shouldSkipStaleUpdate(currentRateLimit, incoming) {
			currentRateLimit = incoming
		}
	case "graphql":
		if !shouldSkipStaleUpdate(resourceLimits["graphql"], incoming) {
			resourceLimits["graphql"] = incoming
		}
		coreInfo, hasCore := resourceLimits["core"]
		if !hasCore || !coreInfo.HasData {
			if !currentRateLimit.HasData || currentRateLimit.Resource == "graphql" {
				if !shouldSkipStaleUpdate(currentRateLimit, incoming) {
					currentRateLimit = incoming
				}
			}
		}
	default:
		if !shouldSkipStaleUpdate(resourceLimits[resource], incoming) {
			resourceLimits[resource] = incoming
		}
	}
}

// shouldSkipStaleUpdate prevents out-of-order concurrent responses from regressing rate limit state.
func shouldSkipStaleUpdate(existing, incoming RateLimitInfo) bool {
	if !existing.HasData {
		return false
	}
	if !existing.ResetTime.IsZero() && !incoming.ResetTime.IsZero() {
		if incoming.ResetTime.Before(existing.ResetTime) {
			return true
		}
	}
	return existing.Limit == incoming.Limit &&
		existing.ResetTime.Equal(incoming.ResetTime) &&
		incoming.Remaining > existing.Remaining
}

// GetRateLimitInfo retrieves the latest recorded GitHub API rate limit status.
func GetRateLimitInfo() RateLimitInfo {
	rateLimitMu.RLock()
	defer rateLimitMu.RUnlock()
	return currentRateLimit
}

// GetRateLimitForResource retrieves the latest recorded rate limit status for a specific
// GitHub rate limit resource (such as "core", "graphql", or "search").
func GetRateLimitForResource(resource string) RateLimitInfo {
	res := strings.ToLower(strings.TrimSpace(resource))
	if res == "" {
		return GetRateLimitInfo()
	}
	rateLimitMu.RLock()
	defer rateLimitMu.RUnlock()
	return resourceLimits[res]
}

// SetRateLimitInfo allows tests or callers to inject rate limit state.
// Passing a zero-value RateLimitInfo resets both default and per-resource rate limit state.
func SetRateLimitInfo(info RateLimitInfo) {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()

	currentRateLimit = info
	res := strings.ToLower(strings.TrimSpace(info.Resource))
	if !info.HasData || res == "" || res == "core" {
		resourceLimits = map[string]RateLimitInfo{}
		resourceCooldowns = map[string]time.Time{}
		if info.HasData {
			resourceLimits["core"] = info
		}
		return
	}

	delete(resourceCooldowns, res)
	resourceLimits[res] = info
}

// RecordRateLimitCooldown records a temporary cooldown deadline for resource
// (for example from a secondary rate-limit Retry-After or GraphQL RATE_LIMITED response).
func RecordRateLimitCooldown(resource string, until time.Time) {
	if until.IsZero() {
		return
	}
	res := strings.ToLower(strings.TrimSpace(resource))
	if res == "" {
		res = "core"
	}

	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	if existing, ok := resourceCooldowns[res]; !ok || until.After(existing) {
		resourceCooldowns[res] = until
	}
}

// CheckRateLimit verifies whether the given GitHub API rate limit resource currently has
// remaining quota or an active cooldown. If exhausted and the reset window has not yet
// elapsed, it returns an *APIError wrapping ErrRateLimited.
func CheckRateLimit(resource string) error {
	res := strings.ToLower(strings.TrimSpace(resource))
	if res == "" {
		res = "core"
	}

	now := time.Now()

	rateLimitMu.RLock()
	cooldownUntil := resourceCooldowns[res]
	rateLimitMu.RUnlock()

	if !cooldownUntil.IsZero() && now.Before(cooldownUntil) {
		retryAfter := cooldownUntil.Sub(now)
		return &APIError{
			StatusCode:  0,
			Resource:    res,
			Message:     fmt.Sprintf("%s rate limit cooldown active (resets in %s)", res, formatResetCountdown(cooldownUntil)),
			RateLimited: true,
			RetryAfter:  retryAfter,
			ResetAt:     cooldownUntil,
			Err:         ErrRateLimited,
		}
	}

	info := GetRateLimitForResource(res)
	if !info.HasData || info.Remaining > 0 {
		return nil
	}

	var retryAfter time.Duration
	if !info.ResetTime.IsZero() {
		if !now.Before(info.ResetTime) {
			return nil
		}
		retryAfter = info.ResetTime.Sub(now)
	}

	countdown := formatResetCountdown(info.ResetTime)
	return &APIError{
		StatusCode:  0,
		Resource:    res,
		Message:     fmt.Sprintf("%s rate limit exhausted (resets in %s)", res, countdown),
		RateLimited: true,
		RetryAfter:  retryAfter,
		ResetAt:     info.ResetTime,
		Err:         ErrRateLimited,
	}
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
