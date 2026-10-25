//go:build linux || darwin

package filelock

import (
	"errors"
	"os"
	"syscall"
)

func tryLock(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return ErrBusy
		}
		if err != nil {
			return errors.New("cannot acquire process lock")
		}
		return nil
	}
}
