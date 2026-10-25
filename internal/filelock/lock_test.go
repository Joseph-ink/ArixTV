package filelock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLockLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "refresh.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second lock: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal("close not idempotent")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("lock inode must persist")
	}
	next, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
	if _, err := Acquire(filepath.Join(t.TempDir(), "missing", "refresh.lock")); err == nil {
		t.Fatal("missing directory accepted")
	}
}

// A real subprocess holds the lock until killed. Pipes establish ordering;
// no timing assumptions, polling loops or sleeps are used.
func TestLockProcess(t *testing.T) {
	if path := os.Getenv("ARIXTV_TEST_LOCK_PATH"); path != "" {
		lock, err := Acquire(path)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		fmt.Println("locked")
		var b [1]byte
		os.Stdin.Read(b[:])
		return
	}
	path := filepath.Join(t.TempDir(), "refresh.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockProcess$")
	cmd.Env = append(os.Environ(), "ARIXTV_TEST_LOCK_PATH="+path)
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper: %q %v", line, err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("cross-process collision: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("process exit left stale lock: %v", err)
	}
	lock.Close()
}
