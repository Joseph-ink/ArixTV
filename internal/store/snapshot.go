// Package store owns versioned provider snapshots and refresh observations.
package store

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/atomicfile"
	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/filelock"
	"github.com/Joseph-ink/ArixTV/internal/statefile"
)

const maxSnapshotBytes = 16 << 20

type Snapshot struct {
	SchemaVersion int               `json:"schema_version"`
	ProviderID    string            `json:"provider_id"`
	Generation    uint64            `json:"generation"`
	AttemptedAt   time.Time         `json:"attempted_at"`
	CompletedAt   time.Time         `json:"completed_at"`
	Channels      []channel.Channel `json:"channels"`
}

// Candidate must represent a fully completed discovery. A future coordinator
// must leave Complete=false whenever discovery returns an error or partial data.
// No store can infer an upstream omission hidden behind Complete=true.
type Candidate struct {
	ProviderID  string
	AttemptedAt time.Time
	CompletedAt time.Time
	Complete    bool
	Channels    []channel.Channel
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != 1 || !channel.ValidProviderID(s.ProviderID) || s.Generation == 0 || s.AttemptedAt.IsZero() || s.CompletedAt.Before(s.AttemptedAt) || len(s.Channels) == 0 || len(s.Channels) > 100000 {
		return errors.New("invalid snapshot header or empty channel set")
	}
	seen := map[string]bool{}
	for _, c := range s.Channels {
		if c.ProviderID != s.ProviderID || seen[c.ProviderChannelID] || c.LastSeen.After(s.CompletedAt) {
			return errors.New("inconsistent or duplicate snapshot channel")
		}
		if err := c.Validate(); err != nil {
			return err
		}
		seen[c.ProviderChannelID] = true
	}
	return nil
}

type Store struct {
	dataDir  string
	stateDir string
	// Per-instance seam enables deterministic failure tests without global hooks.
	writeJSON func(string, any) error
}

func New(dataDir, stateDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(stateDir) == "" {
		return nil, errors.New("data and state directories are required")
	}
	return &Store{dataDir: dataDir, stateDir: stateDir, writeJSON: atomicfile.WriteJSON}, nil
}
func (s *Store) snapshotPath(id string) string {
	return filepath.Join(s.dataDir, "providers", id+".json")
}
func (s *Store) metadataPath(id string) string {
	return filepath.Join(s.stateDir, "refresh", id+".json")
}

func (s *Store) Load(id string) (Snapshot, error) {
	var snapshot Snapshot
	if strings.TrimSpace(s.dataDir) == "" || strings.TrimSpace(s.stateDir) == "" {
		return snapshot, errors.New("uninitialized snapshot store")
	}
	if !channel.ValidProviderID(id) {
		return snapshot, errors.New("invalid provider ID")
	}
	if err := statefile.ReadJSON(s.snapshotPath(id), maxSnapshotBytes, &snapshot); err != nil {
		return Snapshot{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	if snapshot.ProviderID != id {
		return Snapshot{}, errors.New("snapshot provider mismatch")
	}
	return snapshot, nil
}

// Commit replaces only complete, nonempty, validated results. Corrupt existing
// state is reported for operator recovery, never silently reset or overwritten.
func (s *Store) Commit(c Candidate) (Snapshot, error) {
	if !c.Complete {
		return Snapshot{}, errors.New("discovery did not complete")
	}
	if !channel.ValidProviderID(c.ProviderID) {
		return Snapshot{}, errors.New("invalid provider ID")
	}
	lock, err := s.lock(c.ProviderID)
	if err != nil {
		return Snapshot{}, err
	}
	defer lock.Close()
	old, err := s.Load(c.ProviderID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, err
	}
	if old.Generation == math.MaxUint64 {
		return Snapshot{}, errors.New("snapshot generation overflow")
	}
	if old.Generation > 0 && (!c.CompletedAt.After(old.CompletedAt) || c.AttemptedAt.Before(old.AttemptedAt)) {
		return Snapshot{}, errors.New("stale refresh candidate")
	}
	next := Snapshot{SchemaVersion: 1, ProviderID: c.ProviderID, Generation: old.Generation + 1, AttemptedAt: c.AttemptedAt.UTC(), CompletedAt: c.CompletedAt.UTC(), Channels: c.Channels}
	if err := next.Validate(); err != nil {
		return Snapshot{}, err
	}
	// Bound the serialized file as well as reads; never write a file Load rejects.
	if err := s.writeSnapshot(s.snapshotPath(c.ProviderID), next); err != nil {
		return Snapshot{}, err
	}
	return next, nil
}

func (s *Store) lock(id string) (*filelock.Lock, error) {
	if strings.TrimSpace(s.dataDir) == "" || strings.TrimSpace(s.stateDir) == "" {
		return nil, errors.New("uninitialized snapshot store")
	}
	if err := atomicfile.EnsureDir(filepath.Join(s.dataDir, "providers")); err != nil {
		return nil, errors.New("cannot create snapshot directory")
	}
	return filelock.Acquire(filepath.Join(s.dataDir, "providers", "."+id+".lock"))
}

func metadataFrom(s Snapshot) Metadata {
	completed := s.CompletedAt
	return Metadata{SchemaVersion: 1, ProviderID: s.ProviderID, Generation: s.Generation, LastAttempt: s.AttemptedAt, LastSuccess: &completed, Status: Healthy, ChannelCount: len(s.Channels)}
}

// ObserveAttempt records attempt/failure metadata without touching LKG channels.
// Failure summaries are fixed codes, never untrusted upstream exception text.
func (s *Store) ObserveAttempt(id string, at time.Time, failure Failure) error {
	if !channel.ValidProviderID(id) || at.IsZero() || !failure.Valid() {
		return errors.New("invalid refresh observation")
	}
	lock, err := s.lock(id)
	if err != nil {
		return err
	}
	defer lock.Close()
	snapshot, err := s.Load(id)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m := Metadata{SchemaVersion: 1, ProviderID: id, Status: Unknown}
	if snapshot.Generation > 0 {
		m = metadataFrom(snapshot)
		if at.Before(snapshot.CompletedAt) {
			return errors.New("stale refresh observation")
		}
	}
	// Reject stale observations even when a previous attempt has failed.
	var previous Metadata
	err = statefile.ReadJSON(s.metadataPath(id), 1<<20, &previous)
	if err == nil {
		if previous.ProviderID != id || previous.Validate() != nil {
			return errors.New("invalid stored observation")
		}
		if at.Before(previous.LastAttempt) {
			return errors.New("stale refresh observation")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.LastAttempt = at.UTC()
	m.Status = Refreshing
	m.ErrorSummary = failure
	if failure != "" {
		m.Status = Failed
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if err := atomicfile.EnsureDir(filepath.Join(s.stateDir, "refresh")); err != nil {
		return errors.New("cannot create refresh state directory")
	}
	return s.writeJSON(s.metadataPath(id), m)
}

// Status derives successful generations from the snapshot itself, so a crash
// between a snapshot and observation write cannot misreport its channel count.
func (s *Store) Status(id string) (Metadata, error) {
	snapshot, err := s.Load(id)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Metadata{}, err
	}
	m := Metadata{SchemaVersion: 1, ProviderID: id, Status: Unknown}
	if snapshot.Generation > 0 {
		m = metadataFrom(snapshot)
	}
	var observation Metadata
	err = statefile.ReadJSON(s.metadataPath(id), 1<<20, &observation)
	if errors.Is(err, os.ErrNotExist) {
		if snapshot.Generation == 0 {
			return Metadata{}, os.ErrNotExist
		}
		return m, nil
	}
	if err != nil {
		return Metadata{}, err
	}
	if observation.ProviderID != id || observation.Validate() != nil {
		return Metadata{}, errors.New("invalid refresh observation")
	}
	if observation.Generation > snapshot.Generation {
		return Metadata{}, errors.New("observation references missing generation")
	}
	if observation.Generation == snapshot.Generation && (snapshot.Generation == 0 || !observation.LastAttempt.Before(snapshot.CompletedAt)) {
		// Always derive LKG success/count from the actual snapshot.
		m.LastAttempt = observation.LastAttempt
		m.Status = observation.Status
		m.ErrorSummary = observation.ErrorSummary
	}
	return m, nil
}
