package workerhandler

import (
	"testing"
	"time"
)

func TestOutboxBackoffGrowsExponentially(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 5 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
	}
	for _, tc := range cases {
		if got := outboxBackoff(tc.attempt); got != tc.want {
			t.Errorf("outboxBackoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
	if outboxBackoff(-1) <= 0 {
		t.Error("negative attempt produced a non-positive delay")
	}
}
