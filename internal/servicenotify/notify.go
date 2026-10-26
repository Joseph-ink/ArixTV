// Package servicenotify isolates optional service-manager notifications.
package servicenotify

import (
	"errors"
	"strconv"
	"time"
)

type Notifier interface {
	Notify(string) error
	WatchdogInterval() time.Duration
}
type noop struct{}

func (noop) Notify(string) error             { return nil }
func (noop) WatchdogInterval() time.Duration { return 0 }

func watchdog(usec, pid string, current int) (time.Duration, error) {
	if pid != "" {
		id, err := strconv.Atoi(pid)
		if err != nil || id <= 0 {
			return 0, errors.New("invalid service watchdog PID")
		}
		if id != current {
			return 0, nil
		}
	}
	if usec == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(usec, 10, 64)
	if err != nil || n < 2000 || n > uint64((24*time.Hour)/time.Microsecond) {
		return 0, errors.New("service watchdog period must be between 2ms and 24h")
	}
	return time.Duration(n) * time.Microsecond / 2, nil
}
