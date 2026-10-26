//go:build !linux

package servicenotify

import "testing"

func TestPortableNoop(t *testing.T) {
	n, err := FromEnv()
	if err != nil || n.WatchdogInterval() != 0 || n.Notify("READY=1") != nil {
		t.Fatal("non-Linux notifier is not a no-op")
	}
}
