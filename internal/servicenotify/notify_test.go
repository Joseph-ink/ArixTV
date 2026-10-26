package servicenotify

import (
	"testing"
	"time"
)

func TestWatchdogSettings(t *testing.T) {
	for _, tc := range []struct {
		usec, pid string
		want      time.Duration
		valid     bool
	}{{"", "", 0, true}, {"30000000", "123", 15 * time.Second, true}, {"30000000", "456", 0, true}, {"bad", "", 0, false}, {"0", "", 0, false}, {"30000000", "bad", 0, false}, {"18446744073709551615", "", 0, false}} {
		got, err := watchdog(tc.usec, tc.pid, 123)
		if (err == nil) != tc.valid || got != tc.want {
			t.Fatalf("watchdog settings: %+v %s %v", tc, got, err)
		}
	}
}
