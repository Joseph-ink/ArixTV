package store

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Joseph-ink/ArixTV/internal/atomicfile"
	"github.com/Joseph-ink/ArixTV/internal/filelock"
)

// LockRefresh is the single global lock for future scheduled/manual refresh
// coordinators. Hold it until all snapshot/metadata/playlist work has completed.
func LockRefresh(stateDir string) (*filelock.Lock, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, errors.New("state directory required")
	}
	if err := atomicfile.EnsureDir(stateDir); err != nil {
		return nil, errors.New("cannot create refresh lock directory")
	}
	return filelock.Acquire(filepath.Join(stateDir, "refresh.lock"))
}
