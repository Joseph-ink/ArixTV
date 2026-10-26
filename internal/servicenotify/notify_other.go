//go:build !linux

package servicenotify

// Darwin development never depends on systemd or its environment variables.
func FromEnv() (Notifier, error) { return noop{}, nil }
