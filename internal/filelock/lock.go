// Package filelock provides nonblocking process locks. Lock files persist: never
// unlink a lock file, because doing so can split contenders across two inodes.
package filelock

import (
	"errors"
	"os"
	"sync"
)

var ErrBusy = errors.New("lock is held by another operation")
var ErrUnsupported = errors.New("process locking unsupported on this platform")

type Lock struct {
	mu   sync.Mutex
	file *os.File
}

func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("cannot open lock file")
	}
	if err := tryLock(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{file: f}, nil
}

// Close releases the kernel lock. Idempotent and safe for concurrent callers.
// Process exit also releases it; no stale PID or timeout stealing is needed.
func (l *Lock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	if err != nil {
		return errors.New("cannot close lock file")
	}
	return nil
}
