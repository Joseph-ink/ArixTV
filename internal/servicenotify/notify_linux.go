//go:build linux

package servicenotify

import (
	"errors"
	"net"
	"os"
	"strings"
	"time"
)

type linuxNotifier struct {
	socket   string
	interval time.Duration
}

func FromEnv() (Notifier, error) {
	return fromSettings(os.Getenv("NOTIFY_SOCKET"), os.Getenv("WATCHDOG_USEC"), os.Getenv("WATCHDOG_PID"), os.Getpid())
}
func fromSettings(socket, usec, pid string, current int) (Notifier, error) {
	if socket == "" {
		return noop{}, nil
	}
	if len(socket) > 107 || len(socket) < 2 || !(strings.HasPrefix(socket, "/") || strings.HasPrefix(socket, "@")) || strings.ContainsRune(socket, 0) {
		return nil, errors.New("invalid service notification socket")
	}
	period, err := watchdog(usec, pid, current)
	if err != nil {
		return nil, err
	}
	return &linuxNotifier{socket: socket, interval: period}, nil
}
func (n *linuxNotifier) WatchdogInterval() time.Duration { return n.interval }
func (n *linuxNotifier) Notify(state string) error {
	if state != "READY=1" && state != "WATCHDOG=1" && state != "STOPPING=1" {
		return errors.New("unsupported service notification")
	}
	conn, err := net.DialTimeout("unixgram", n.socket, 200*time.Millisecond)
	if err != nil {
		return errors.New("service notification unavailable")
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		return errors.New("service notification deadline failed")
	}
	if _, err := conn.Write([]byte(state)); err != nil {
		return errors.New("service notification failed")
	}
	return nil
}
