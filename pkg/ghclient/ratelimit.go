package ghclient

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultUnknownResetProbe = 30 * time.Second

// RateLimitInfo holds the current state of GitHub API rate limits.
type RateLimitInfo struct {
	Resource  string
	Limit     int
	Remaining int
	ResetTime time.Time
	HasData   bool
}

type rateLimitBucket struct {
	info            RateLimitInfo
	cooldownUntil   time.Time
	unknownProbeEnd time.Time
}

var (
	rateLimitMu      sync.RWMutex
	currentRateLimit RateLimitInfo
	resourceLimits   = map[string]RateLimitInfo{}
	resourceBuckets  = map[string]*rateLimitBucket{}
)

// UpdateRateLimitFromHeaders extracts GitHub rate limit headers from HTTP responses.
func UpdateRateLimitFromHeaders(headers http.Header) {
	UpdateRateLimitFromHeadersScoped("", headers, "")
}

// UpdateRateLimitFromHeadersScoped extracts GitHub rate limit headers from HTTP responses
// for a specific quota owner scope (for example API base URL + credential fingerprint)
// and an optional default resource when X-Ratelimit-Resource is absent.
func UpdateRateLimitFromHeadersScoped(ownerScope string, headers http.Header, defaultResource string) {
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

	rawResource := strings.ToLower(strings.TrimSpace(headers.Get("X-Ratelimit-Resource")))
	resource := rawResource
	if resource == "" {
		resource = strings.ToLower(strings.TrimSpace(defaultResource))
	}

	incoming := RateLimitInfo{
		Resource:  resource,
		Limit:     lim,
		Remaining: rem,
		ResetTime: resetTime,
		HasData:   true,
	}

	now := time.Now()
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()

	if ownerScope != "" {
		scopedRes := resource
		if scopedRes == "" {
			scopedRes = "core"
		}
		scopedKey := makeRateLimitKey(ownerScope, scopedRes)
		b := getOrCreateBucketLocked(scopedKey)
		if !shouldSkipStaleUpdate(b.info, incoming) {
			b.info = incoming
			updateProbeDeadlineLocked(b, incoming, now)
		}
	}

	// Also maintain global/status-bar views when unscoped or targeting the default GitHub API.
	if ownerScope == "" || isPrimaryGitHubScope(ownerScope) {
		updateGlobalRateLimitLocked(resource, incoming, now)
	}
}

func updateGlobalRateLimitLocked(resource string, incoming RateLimitInfo, now time.Time) {
	switch resource {
	case "":
		coreIncoming := incoming
		coreIncoming.Resource = "core"
		coreBucket := getOrCreateBucketLocked("core")
		if shouldSkipStaleUpdate(coreBucket.info, coreIncoming) {
			return
		}
		coreBucket.info = coreIncoming
		updateProbeDeadlineLocked(coreBucket, coreIncoming, now)
		resourceLimits["core"] = coreIncoming

		if !currentRateLimit.HasData || currentRateLimit.Resource == "" || currentRateLimit.Resource == "core" || currentRateLimit.Resource == "graphql" {
			currentRateLimit = incoming
		}
	case "core":
		coreBucket := getOrCreateBucketLocked("core")
		if !shouldSkipStaleUpdate(coreBucket.info, incoming) {
			coreBucket.info = incoming
			updateProbeDeadlineLocked(coreBucket, incoming, now)
			resourceLimits["core"] = incoming

			isCoreOrFallback := !currentRateLimit.HasData || currentRateLimit.Resource == "" || currentRateLimit.Resource == "core" || currentRateLimit.Resource == "graphql"
			if isCoreOrFallback {
				currentRateLimit = incoming
			}
		}
	case "graphql":
		gqlBucket := getOrCreateBucketLocked("graphql")
		if !shouldSkipStaleUpdate(gqlBucket.info, incoming) {
			gqlBucket.info = incoming
			updateProbeDeadlineLocked(gqlBucket, incoming, now)
			resourceLimits["graphql"] = incoming
		}
		coreInfo := resourceLimits["core"]
		if !coreInfo.HasData {
			if !currentRateLimit.HasData || currentRateLimit.Resource == "graphql" {
				if !shouldSkipStaleUpdate(currentRateLimit, incoming) {
					currentRateLimit = incoming
				}
			}
		}
	default:
		b := getOrCreateBucketLocked(resource)
		if !shouldSkipStaleUpdate(b.info, incoming) {
			b.info = incoming
			updateProbeDeadlineLocked(b, incoming, now)
			resourceLimits[resource] = incoming
		}
	}
}

func updateProbeDeadlineLocked(b *rateLimitBucket, info RateLimitInfo, now time.Time) {
	if info.Remaining == 0 && info.ResetTime.IsZero() {
		b.unknownProbeEnd = now.Add(defaultUnknownResetProbe)
		return
	}
	b.unknownProbeEnd = time.Time{}
}

func getOrCreateBucketLocked(key string) *rateLimitBucket {
	b, ok := resourceBuckets[key]
	if !ok {
		b = &rateLimitBucket{}
		resourceBuckets[key] = b
	}
	return b
}

func makeRateLimitKey(ownerScope, resource string) string {
	res := strings.ToLower(strings.TrimSpace(resource))
	if res == "" {
		res = "core"
	}
	if ownerScope == "" {
		return res
	}
	return ownerScope + "|" + res
}

func isPrimaryGitHubScope(ownerScope string) bool {
	return strings.HasPrefix(ownerScope, defaultAPIBaseURL+"|") ||
		strings.HasPrefix(ownerScope, defaultGraphQLURL+"|")
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

// HasActiveRateLimitState reports whether quota data or an active cooldown exists
// for the given owner scope and resource.
func HasActiveRateLimitState(ownerScope, resource string) bool {
	key := makeRateLimitKey(ownerScope, resource)
	rateLimitMu.RLock()
	defer rateLimitMu.RUnlock()
	b, ok := resourceBuckets[key]
	if !ok {
		return false
	}
	return b.info.HasData || !b.cooldownUntil.IsZero()
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
		resourceBuckets = map[string]*rateLimitBucket{}
		if info.HasData {
			resourceLimits["core"] = info
			b := &rateLimitBucket{info: info}
			updateProbeDeadlineLocked(b, info, time.Now())
			resourceBuckets["core"] = b
		}
		return
	}

	resourceLimits[res] = info
	b := &rateLimitBucket{info: info}
	updateProbeDeadlineLocked(b, info, time.Now())
	resourceBuckets[res] = b
}

// RecordRateLimitCooldown records a temporary cooldown deadline for resource.
func RecordRateLimitCooldown(resource string, until time.Time) {
	RecordRateLimitCooldownScoped("", resource, until)
}

// RecordRateLimitCooldownScoped records a temporary cooldown deadline scoped to a quota owner.
func RecordRateLimitCooldownScoped(ownerScope, resource string, until time.Time) {
	if until.IsZero() {
		return
	}
	res := strings.ToLower(strings.TrimSpace(resource))
	if res == "" {
		res = "core"
	}

	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()

	key := makeRateLimitKey(ownerScope, res)
	b := getOrCreateBucketLocked(key)
	if until.After(b.cooldownUntil) {
		b.cooldownUntil = until
	}
	if ownerScope != "" && isPrimaryGitHubScope(ownerScope) {
		globalB := getOrCreateBucketLocked(res)
		if until.After(globalB.cooldownUntil) {
			globalB.cooldownUntil = until
		}
	}
}

// CheckRateLimit verifies whether the given GitHub API rate limit resource currently has
// remaining quota or an active cooldown.
func CheckRateLimit(resource string) error {
	return CheckRateLimitScoped("", resource)
}

// CheckRateLimitScoped verifies quota and cooldown state for a specific owner scope and resource
// in a single locked snapshot.
func CheckRateLimitScoped(ownerScope, resource string) error {
	res := strings.ToLower(strings.TrimSpace(resource))
	if res == "" {
		res = "core"
	}
	key := makeRateLimitKey(ownerScope, res)
	now := time.Now()

	rateLimitMu.Lock()
	b := resourceBuckets[key]
	if b == nil {
		rateLimitMu.Unlock()
		return nil
	}
	cooldownUntil := b.cooldownUntil
	info := b.info
	probeEnd := b.unknownProbeEnd
	if info.HasData && info.Remaining == 0 && info.ResetTime.IsZero() && probeEnd.IsZero() {
		probeEnd = now.Add(defaultUnknownResetProbe)
		b.unknownProbeEnd = probeEnd
	}
	rateLimitMu.Unlock()

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

	if !info.HasData || info.Remaining > 0 {
		return nil
	}

	if !info.ResetTime.IsZero() {
		if !now.Before(info.ResetTime) {
			return nil
		}
		return &APIError{
			StatusCode:  0,
			Resource:    res,
			Message:     fmt.Sprintf("%s rate limit exhausted (resets in %s)", res, formatResetCountdown(info.ResetTime)),
			RateLimited: true,
			RetryAfter:  info.ResetTime.Sub(now),
			ResetAt:     info.ResetTime,
			Err:         ErrRateLimited,
		}
	}

	if now.Before(probeEnd) {
		return &APIError{
			StatusCode:  0,
			Resource:    res,
			Message:     fmt.Sprintf("%s rate limit exhausted (probe allowed in %s)", res, formatResetCountdown(probeEnd)),
			RateLimited: true,
			RetryAfter:  probeEnd.Sub(now),
			ResetAt:     probeEnd,
			Err:         ErrRateLimited,
		}
	}

	return nil
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
