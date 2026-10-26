//go:build linux

package servicenotify

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestLinuxNotifications(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenPacket("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	n, err := fromSettings(path, "30000000", "123", 123)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"READY=1", "WATCHDOG=1", "STOPPING=1"} {
		if err := n.Notify(state); err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(time.Second))
		var b [64]byte
		count, _, err := conn.ReadFrom(b[:])
		if err != nil || string(b[:count]) != state {
			t.Fatalf("notification %q %v", b[:count], err)
		}
	}
	if n.WatchdogInterval() != 15*time.Second {
		t.Fatal("watchdog interval")
	}
	if n.Notify("PRIVATE_PLACEHOLDER") == nil {
		t.Fatal("arbitrary notification accepted")
	}
	if _, err := fromSettings("relative.sock", "", "", 123); err == nil {
		t.Fatal("relative socket accepted")
	}
}
