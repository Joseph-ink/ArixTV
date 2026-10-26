// Package provider defines provider metadata and the immutable runtime registry.
package provider

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

type Status string

const (
	Stable       Status = "stable"
	Experimental Status = "experimental"
	Disabled     Status = "disabled"
	Unsupported  Status = "unsupported"
)

func (s Status) Valid() bool {
	return s == Stable || s == Experimental || s == Disabled || s == Unsupported
}

type Access string

const (
	AnonymousFree Access = "anonymous_free"
	FreeAccount   Access = "free_account"
	PaidAccount   Access = "paid_account"
	TVEverywhere  Access = "tv_everywhere"
	UnknownAccess Access = "unknown"
)

func (a Access) Valid() bool {
	return a == AnonymousFree || a == FreeAccount || a == PaidAccount || a == TVEverywhere || a == UnknownAccess
}

type Metadata struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Status           Status               `json:"status"`
	Access           Access               `json:"access"`
	UnofficialMirror bool                 `json:"unofficial_mirror"`
	Capabilities     []channel.Capability `json:"capabilities,omitempty"`
}

func (m Metadata) Validate() error {
	if !channel.ValidProviderID(m.ID) || strings.TrimSpace(m.Name) == "" || len(m.Name) > 256 || !m.Status.Valid() || !m.Access.Valid() {
		return errors.New("invalid provider metadata")
	}
	if m.Status == Stable && (m.UnofficialMirror || m.Access != AnonymousFree) {
		return errors.New("stable V1 providers require anonymous access and provider-owned public sources")
	}
	seen := map[channel.Capability]bool{}
	for _, c := range m.Capabilities {
		if !c.Valid() || seen[c] {
			return errors.New("invalid provider capabilities")
		}
		seen[c] = true
	}
	return nil
}

// Provider only discovers identities and resolves ephemeral playback data.
// An error means discovery is incomplete and must not be committed as a snapshot.
type Provider interface {
	Discover(context.Context) ([]channel.Channel, error)
	Resolve(context.Context, channel.Channel) (channel.Stream, error)
}

type Entry struct {
	Metadata Metadata
	Runtime  Provider
}

// Registry is immutable after construction and safe for concurrent reads.
type Registry struct{ entries map[string]Entry }

func NewRegistry(entries ...Entry) (*Registry, error) {
	r := &Registry{entries: make(map[string]Entry, len(entries))}
	for _, e := range entries {
		if err := e.Metadata.Validate(); err != nil {
			return nil, err
		}
		if _, exists := r.entries[e.Metadata.ID]; exists {
			return nil, errors.New("duplicate provider ID")
		}
		if e.Runtime != nil {
			v := reflect.ValueOf(e.Runtime)
			if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Map || v.Kind() == reflect.Slice || v.Kind() == reflect.Func || v.Kind() == reflect.Interface || v.Kind() == reflect.Chan) && v.IsNil() {
				return nil, errors.New("nil provider runtime")
			}
		}
		e.Metadata.Capabilities = append([]channel.Capability(nil), e.Metadata.Capabilities...)
		r.entries[e.Metadata.ID] = e
	}
	return r, nil
}

func (r *Registry) Lookup(id string) (Metadata, Provider, bool) {
	e, ok := r.entries[id]
	e.Metadata.Capabilities = append([]channel.Capability(nil), e.Metadata.Capabilities...)
	return e.Metadata, e.Runtime, ok
}

func (r *Registry) List() []Metadata {
	result := make([]Metadata, 0, len(r.entries))
	for id := range r.entries {
		m, _, _ := r.Lookup(id)
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// ValidateEnabled checks operational availability; CLI config validation keeps
// its backwards-compatible syntax-only provider ID behavior.
func (r *Registry) ValidateEnabled(ids []string) error {
	seen := map[string]bool{}
	for _, id := range ids {
		m, p, ok := r.Lookup(id)
		if seen[id] || !ok || p == nil || m.Access != AnonymousFree || (m.Status != Stable && m.Status != Experimental) {
			return errors.New("provider is unavailable or outside anonymous V1 policy")
		}
		seen[id] = true
	}
	return nil
}
