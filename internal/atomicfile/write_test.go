package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type failingFile struct {
	*os.File
	stage string
}

func (f failingFile) Write(b []byte) (int, error) {
	if f.stage == "write" {
		return 0, errors.New("injected")
	}
	if f.stage == "short" {
		return 0, nil
	}
	return f.File.Write(b)
}
func (f failingFile) Sync() error {
	if f.stage == "sync" {
		return errors.New("injected")
	}
	return f.File.Sync()
}
func (f failingFile) Close() error {
	err := f.File.Close()
	if f.stage == "close" {
		return errors.New("injected")
	}
	return err
}

type failingDir struct{ *os.File }

func (f failingDir) Sync() error { return errors.New("injected") }

func TestReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "playlist.m3u")
	if err := Write(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	oldReader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer oldReader.Close()
	if err := Write(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	oldBytes, err := io.ReadAll(oldReader)
	if err != nil || string(oldBytes) != "old" {
		t.Fatal("replacement modified an existing reader's inode")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "new" {
		t.Fatal("replacement failed")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("file not private")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary file left behind")
	}
	if err := WriteJSON(path, make(chan int)); err == nil {
		t.Fatal("serialization error ignored")
	}
	b, _ = os.ReadFile(path)
	if string(b) != "new" {
		t.Fatal("serialization failure changed old state")
	}
}

func TestEnsureDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "data", "providers")
	if err := EnsureDir(path); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(path); err != nil {
		t.Fatal("existing directory failed")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0700 {
		t.Fatal("directory not private")
	}
	file := filepath.Join(root, "file")
	os.WriteFile(file, []byte("existing"), 0600)
	if EnsureDir(filepath.Join(file, "providers")) == nil {
		t.Fatal("file accepted as parent")
	}
}

func TestFailureBoundaries(t *testing.T) {
	for _, stage := range []string{"open", "create", "write", "short", "sync", "close", "rename", "directory_sync"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "snapshot.json")
			os.WriteFile(path, []byte("old"), 0600)
			err := replace(path, []byte("new"), operations{
				create: func(dir string) (tempFile, error) {
					if stage == "create" {
						return nil, errors.New("injected")
					}
					f, err := os.CreateTemp(dir, ".arixtv-*")
					return failingFile{f, stage}, err
				},
				openDir: func(dir string) (directory, error) {
					if stage == "open" {
						return nil, errors.New("injected")
					}
					f, err := os.Open(dir)
					if stage == "directory_sync" {
						return failingDir{f}, err
					}
					return f, err
				},
				rename: func(a, b string) error {
					if stage == "rename" {
						return errors.New("injected")
					}
					return os.Rename(a, b)
				},
			})
			if err == nil {
				t.Fatal("injected failure not reported")
			}
			b, _ := os.ReadFile(path)
			if stage == "directory_sync" {
				var uncertain *DurabilityError
				if !errors.As(err, &uncertain) || string(b) != "new" {
					t.Fatal("lost post-commit signal")
				}
			} else if string(b) != "old" {
				t.Fatal("pre-commit failure destroyed LKG")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 {
				t.Fatal("orphan temporary file")
			}
		})
	}
}
