package ghclient

import (
	"net/http"
	"testing"
	"time"
)

func TestUpdateRateLimitFromHeaders(t *testing.T) {
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
