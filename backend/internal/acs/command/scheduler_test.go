package command

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	scheduler := NewScheduler(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute, []time.Duration{time.Second, 5 * time.Second})
	tests := []struct {
		attempts int
		want     time.Duration
	}{{0, time.Second}, {1, time.Second}, {2, 5 * time.Second}, {8, 5 * time.Second}}
	for _, test := range tests {
		if got := scheduler.retryDelay(test.attempts); got != test.want {
			t.Fatalf("retryDelay(%d) = %v, want %v", test.attempts, got, test.want)
		}
	}
}
