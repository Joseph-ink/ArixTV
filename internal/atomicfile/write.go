// Package atomicfile persists private local files by sync + same-directory rename.
// The destination directory must already exist on a local filesystem supporting
// atomic rename and advisory locks. It must be controlled by the operator.
package atomicfile

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// DurabilityError means replacement has committed, but directory sync failed.
// The caller must reload/inspect the new file rather than retry assuming the old
// value survived. No implementation can promise rollback after atomic rename.
type DurabilityError struct{}

func (*DurabilityError) Error() string { return "file replaced but directory durability is uncertain" }

type tempFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}
type directory interface {
	Sync() error
	Close() error
}
type operations struct {
	create  func(string) (tempFile, error)
	openDir func(string) (directory, error)
	rename  func(string, string) error
}

func Write(path string, data []byte) error {
	return replace(path, data, operations{
		create:  func(dir string) (tempFile, error) { return os.CreateTemp(dir, ".arixtv-*") },
		openDir: func(dir string) (directory, error) { return os.Open(dir) },
		rename:  os.Rename,
	})
}

func WriteJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return errors.New("cannot serialize state")
	}
	return Write(path, append(data, '\n'))
}

func replace(path string, data []byte, ops operations) error {
	dir, err := ops.openDir(filepath.Dir(path))
	if err != nil {
		return errors.New("cannot open destination directory")
	}
	defer dir.Close()
	f, err := ops.create(filepath.Dir(path))
	if err != nil {
		return errors.New("cannot create temporary state file")
	}
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
		os.Remove(f.Name())
	}()
	if n, err := f.Write(data); err != nil || n != len(data) {
		return errors.New("cannot write complete state file")
	}
	if err := f.Sync(); err != nil {
		return errors.New("cannot sync state file")
	}
	err = f.Close()
	closed = true
	if err != nil {
		return errors.New("cannot close state file")
	}
	if err := ops.rename(f.Name(), path); err != nil {
		return errors.New("cannot replace state file")
	}
	if err := dir.Sync(); err != nil {
		return &DurabilityError{}
	}
	return nil
}
