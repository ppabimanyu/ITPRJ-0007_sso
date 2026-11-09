package repository

import (
	"testing"
	"time"
)

func TestRateLimitDecision(t *testing.T) {
	window := time.Minute
	cases := []struct {
		name      string
		count     int64
		ttl       time.Duration
		limit     int
		allowed   bool
		remaining int
	}{
		{"first request", 1, 60 * time.Second, 5, true, 4},
		{"last allowed", 5, 30 * time.Second, 5, true, 0},
		{"over limit", 6, 20 * time.Second, 5, false, 0},
		{"missing ttl uses window", 1, 0, 5, true, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rateLimitDecision(tc.count, tc.ttl, tc.limit, window)
			if got.Allowed != tc.allowed {
				t.Errorf("Allowed = %v, want %v", got.Allowed, tc.allowed)
			}
			if got.Remaining != tc.remaining {
				t.Errorf("Remaining = %d, want %d", got.Remaining, tc.remaining)
			}
			if tc.ttl == 0 && got.RetryAfter != window {
				t.Errorf("RetryAfter = %v, want %v", got.RetryAfter, window)
			}
		})
	}
}
