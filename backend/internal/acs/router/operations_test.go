package router

import (
	"testing"
	"time"
)

func TestConnectivityStatus(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	online := 10 * time.Minute
	offline := 24 * time.Hour
	tests := []struct {
		name string
		last *time.Time
		want string
	}{
		{"never", nil, "NEVER_CONNECTED"},
		{"online", timePointer(now.Add(-time.Minute)), "ONLINE"},
		{"stale", timePointer(now.Add(-time.Hour)), "STALE"},
		{"offline", timePointer(now.Add(-25 * time.Hour)), "OFFLINE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ConnectivityStatus(test.last, now, online, offline); got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func timePointer(value time.Time) *time.Time { return &value }
