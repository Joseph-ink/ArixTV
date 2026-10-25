package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/filelock"
)

func candidate(id string) Candidate {
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	return Candidate{ProviderID: id, AttemptedAt: now, CompletedAt: now.Add(time.Minute), Complete: true, Channels: []channel.Channel{{ProviderID: id, ProviderChannelID: "one", Name: "Example", StreamType: channel.HLS, DRM: channel.Clear, Capability: channel.ClearDirect, LastSeen: now}}}
}
func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "data"), filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSnapshotLifecycle(t *testing.T) {
	s := testStore(t)
	a := candidate("example")
	saved, err := s.Commit(a)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Generation != 1 || saved.SchemaVersion != 1 {
		t.Fatal("missing generation")
	}
	loaded, err := s.Load("example")
	if err != nil || !reflect.DeepEqual(loaded, saved) {
		t.Fatal("snapshot round trip", err)
	}
	m, err := s.Status("example")
	if err != nil || m.Status != Healthy || m.Generation != 1 || m.ChannelCount != 1 || m.LastSuccess == nil {
		t.Fatalf("success metadata: %+v %v", m, err)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(m)
	var round Metadata
	if err := json.Unmarshal(b, &round); err != nil || !reflect.DeepEqual(m, round) {
		t.Fatal("metadata round trip", err)
	}
	if err := s.ObserveAttempt("example", a.CompletedAt.Add(time.Second), DiscoveryFailure); err != nil {
		t.Fatal(err)
	}
	m, err = s.Status("example")
	if err != nil || m.Status != Failed || m.Generation != 1 || m.ChannelCount != 1 || m.ErrorSummary != DiscoveryFailure {
		t.Fatalf("failure metadata: %+v %v", m, err)
	}
	still, _ := s.Load("example")
	if !reflect.DeepEqual(saved, still) {
		t.Fatal("failed attempt altered LKG")
	}
	a.AttemptedAt = a.CompletedAt.Add(2 * time.Second)
	a.CompletedAt = a.AttemptedAt.Add(time.Minute)
	saved, err = s.Commit(a)
	if err != nil || saved.Generation != 2 {
		t.Fatal("second generation", err)
	}
	m, err = s.Status("example")
	if err != nil || m.Status != Healthy || m.Generation != 2 || m.ErrorSummary != "" {
		t.Fatal("old observation masked success", err)
	}
	info, _ := os.Stat(s.snapshotPath("example"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("snapshot permissions")
	}
}

func TestCandidateFailuresPreserveLKG(t *testing.T) {
	for name, change := range map[string]func(*Candidate){
		"partial":        func(c *Candidate) { c.Complete = false },
		"empty":          func(c *Candidate) { c.Channels = nil },
		"mixed provider": func(c *Candidate) { c.Channels[0].ProviderID = "other" },
		"duplicate":      func(c *Candidate) { c.Channels = append(c.Channels, c.Channels[0]) },
		"unknown enum":   func(c *Candidate) { c.Channels[0].DRM = "wrong" },
		"unsafe direct":  func(c *Candidate) { c.Channels[0].DRM = channel.Widevine },
		"future seen":    func(c *Candidate) { c.Channels[0].LastSeen = c.CompletedAt.Add(time.Hour) },
		"stale":          func(c *Candidate) { c.CompletedAt = c.CompletedAt.Add(-time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			old := candidate("example")
			if _, err := s.Commit(old); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(s.snapshotPath("example"))
			next := candidate("example")
			next.CompletedAt = next.CompletedAt.Add(time.Hour)
			change(&next)
			if _, err := s.Commit(next); err == nil {
				t.Fatal("invalid candidate committed")
			}
			after, _ := os.ReadFile(s.snapshotPath("example"))
			if !bytes.Equal(before, after) {
				t.Fatal("invalid candidate erased LKG")
			}
		})
	}
}

func TestWriteFailureAndProviderIsolation(t *testing.T) {
	s := testStore(t)
	if _, err := s.Commit(candidate("example")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.snapshotPath("example"))
	s.writeJSON = func(string, any) error { return errors.New("injected precommit failure") }
	next := candidate("example")
	next.CompletedAt = next.CompletedAt.Add(time.Hour)
	if _, err := s.Commit(next); err == nil {
		t.Fatal("write failure ignored")
	}
	after, _ := os.ReadFile(s.snapshotPath("example"))
	if !bytes.Equal(before, after) {
		t.Fatal("write failure changed LKG")
	}
	// A new store is not globally poisoned by one provider's failure.
	other, err := New(s.dataDir, s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(candidate("another")); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Load("example"); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptSnapshotAndTraversal(t *testing.T) {
	s := testStore(t)
	if _, err := s.Commit(candidate("example")); err != nil {
		t.Fatal(err)
	}
	valid, _ := os.ReadFile(s.snapshotPath("example"))
	for _, bad := range [][]byte{[]byte("{"), []byte(`{"schema_version":2}`), bytes.Replace(valid, []byte(`"provider_id":"example"`), []byte(`"provider_id":"other"`), 1), bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1)} {
		os.WriteFile(s.snapshotPath("example"), bad, 0600)
		if _, err := s.Load("example"); err == nil {
			t.Fatal("corrupt snapshot accepted")
		}
		next := candidate("example")
		next.CompletedAt = next.CompletedAt.Add(time.Hour)
		if _, err := s.Commit(next); err == nil {
			t.Fatal("corrupt snapshot silently reset")
		}
		current, _ := os.ReadFile(s.snapshotPath("example"))
		if !bytes.Equal(current, bad) {
			t.Fatal("corrupt snapshot overwritten")
		}
	}
	for _, id := range []string{"../example", "/tmp/example", "", "example/other"} {
		if _, err := s.Load(id); err == nil {
			t.Fatal("path traversal accepted")
		}
		if err := s.ObserveAttempt(id, candidate("example").AttemptedAt, DiscoveryFailure); err == nil {
			t.Fatal("invalid observation ID accepted")
		}
	}
	if _, err := New("", s.stateDir); err == nil {
		t.Fatal("empty data dir accepted")
	}
}

func TestInitialAndStaleObservations(t *testing.T) {
	s := testStore(t)
	at := candidate("example").AttemptedAt
	if err := s.ObserveAttempt("example", at, DiscoveryFailure); err != nil {
		t.Fatal(err)
	}
	m, err := s.Status("example")
	if err != nil || m.Generation != 0 || m.LastSuccess != nil || m.ChannelCount != 0 || m.Status != Failed {
		t.Fatalf("initial failure: %+v %v", m, err)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("example"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed discovery wrote snapshot")
	}
	if err := s.ObserveAttempt("example", at.Add(time.Minute), ""); err != nil {
		t.Fatal(err)
	}
	m, _ = s.Status("example")
	if m.Status != Refreshing || !m.LastAttempt.Equal(at.Add(time.Minute)) {
		t.Fatal("attempt not recorded")
	}
	if err := s.ObserveAttempt("example", at, DiscoveryFailure); err == nil {
		t.Fatal("stale metadata overwritten")
	}
	if err := s.ObserveAttempt("example", at, Failure("SECRET_PLACEHOLDER")); err == nil {
		t.Fatal("arbitrary error secret persisted")
	}
	for _, status := range []Health{Healthy, Refreshing, Failed, Stale, Unknown} {
		if !status.Valid() {
			t.Fatal(status)
		}
	}
	if Health("").Valid() {
		t.Fatal("zero health accepted")
	}
}

func TestRefreshAndMutationLocks(t *testing.T) {
	s := testStore(t)
	first, err := LockRefresh(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockRefresh(s.stateDir); !errors.Is(err, filelock.ErrBusy) {
		t.Fatal("global refresh collision not prevented", err)
	}
	first.Close()
	second, err := LockRefresh(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
	lock, err := s.lock("example")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := s.Commit(candidate("example")); !errors.Is(err, filelock.ErrBusy) {
		t.Fatal("snapshot ignored process lock", err)
	}
	if err := s.ObserveAttempt("example", candidate("example").AttemptedAt, DiscoveryFailure); !errors.Is(err, filelock.ErrBusy) {
		t.Fatal("metadata ignored process lock", err)
	}
	if _, err := LockRefresh(""); err == nil {
		t.Fatal("empty refresh root accepted")
	}
}
