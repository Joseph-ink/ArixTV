//go:build !linux && !darwin

package filelock

import "os"

func tryLock(*os.File) error { return ErrUnsupported }
