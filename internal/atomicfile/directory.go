package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
)

// EnsureDir creates private directories and syncs each new directory entry in
// its parent. Existing operator-owned permissions are never changed. This also
// makes the first write's newly created parent chain durable before file commit.
func EnsureDir(path string) error {
	if path == "" {
		return errors.New("storage directory required")
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return errors.New("storage path is not a directory")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot inspect storage directory")
	}
	parent := filepath.Dir(path)
	if parent == path {
		return errors.New("invalid storage directory")
	}
	if err := EnsureDir(parent); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.New("cannot create storage directory")
	}
	info, err = os.Stat(path)
	if err != nil || !info.IsDir() {
		return errors.New("storage directory creation conflict")
	}
	dir, err := os.Open(parent)
	if err != nil {
		return errors.New("cannot open storage parent")
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return errors.New("cannot sync storage directory creation")
	}
	return nil
}
