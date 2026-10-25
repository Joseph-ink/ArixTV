package store

import (
	"errors"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

type Health string

const (
	Healthy    Health = "healthy"
	Refreshing Health = "refreshing"
	Failed     Health = "failed"
	Stale      Health = "stale"
	Unknown    Health = "unknown"
)

func (h Health) Valid() bool {
	return h == Healthy || h == Refreshing || h == Failed || h == Stale || h == Unknown
}

// Failure is a bounded safe error summary, not arbitrary upstream error text.
type Failure string

const (
	DiscoveryFailure   Failure = "discovery_failed"
	ValidationFailure  Failure = "invalid_candidate"
	PersistenceFailure Failure = "persistence_failed"
)

func (f Failure) Valid() bool {
	return f == "" || f == DiscoveryFailure || f == ValidationFailure || f == PersistenceFailure
}

type Metadata struct {
	SchemaVersion int        `json:"schema_version"`
	ProviderID    string     `json:"provider_id"`
	LastAttempt   time.Time  `json:"last_refresh_attempt"`
	LastSuccess   *time.Time `json:"last_successful_refresh,omitempty"`
	Status        Health     `json:"status"`
	ErrorSummary  Failure    `json:"error_summary,omitempty"`
	ChannelCount  int        `json:"channel_count"`
	Generation    uint64     `json:"snapshot_generation"`
}

func (m Metadata) Validate() error {
	if m.SchemaVersion != 1 || !channel.ValidProviderID(m.ProviderID) || m.LastAttempt.IsZero() || !m.Status.Valid() || !m.ErrorSummary.Valid() || m.ChannelCount < 0 {
		return errors.New("invalid refresh metadata")
	}
	if (m.Generation == 0) != (m.LastSuccess == nil) || (m.Generation == 0 && m.ChannelCount != 0) || (m.Generation > 0 && m.ChannelCount == 0) {
		return errors.New("inconsistent refresh generation")
	}
	if m.LastSuccess != nil && m.LastSuccess.IsZero() {
		return errors.New("invalid refresh success timestamp")
	}
	if (m.Status == Healthy || m.Status == Stale) && m.Generation == 0 {
		return errors.New("healthy or stale state requires a successful generation")
	}
	if m.LastSuccess != nil && (m.Status == Failed || m.Status == Refreshing) && m.LastAttempt.Before(*m.LastSuccess) {
		return errors.New("attempt precedes the last successful generation")
	}
	if (m.Status == Failed) != (m.ErrorSummary != "") {
		return errors.New("inconsistent refresh error summary")
	}
	return nil
}
