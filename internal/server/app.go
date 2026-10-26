// Package server implements HTTP control-plane serving. It never fetches media.
package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/auth"
	"github.com/Joseph-ink/ArixTV/internal/config"
	"github.com/Joseph-ink/ArixTV/internal/logsafe"
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/store"
)

// InternalAuth deliberately differs from auth.Store: public tokens must never
// be interchangeable with internal credentials. Nil disables internal routes.
type InternalAuth interface{ VerifyInternal(string) bool }
type internalDigest [32]byte

func (h internalDigest) VerifyInternal(value string) bool {
	sum := sha256.Sum256([]byte(value))
	return subtle.ConstantTimeCompare(h[:], sum[:]) == 1
}
func InternalDigest(hash [32]byte) InternalAuth { return internalDigest(hash) }

type Options struct {
	Registry     *provider.Registry
	InternalAuth InternalAuth
	LogOutput    io.Writer
	Version      string
}
type App struct {
	cfg                         config.Config
	registry                    *provider.Registry
	tokens                      *auth.Store
	internal                    InternalAuth
	logger                      *slog.Logger
	version                     string
	lanNetworks, trustedProxies []netip.Prefix
	catalog                     *catalog
	limiter                     *limiter
	resolvers                   chan struct{}
	connections                 chan struct{}
	now                         func() time.Time
	stopping                    atomic.Bool
	serving                     atomic.Bool
	listen                      func(string, string) (net.Listener, error)
	reloadTicks                 <-chan time.Time // Optional deterministic clock seam for lifecycle tests.
}

func New(c config.Config, options Options) (*App, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if options.Registry == nil {
		return nil, errors.New("provider registry required")
	}
	if err := c.ValidateProviders(options.Registry); err != nil {
		return nil, err
	}
	// Copy caller-owned configuration slices before making handlers concurrent.
	c.Providers.Enabled = append([]string(nil), c.Providers.Enabled...)
	sort.Strings(c.Providers.Enabled)
	c.LAN.ListenAddresses = append([]string(nil), c.LAN.ListenAddresses...)
	c.LAN.AllowNetworks = append([]string(nil), c.LAN.AllowNetworks...)
	c.Remote.TrustedProxies = append([]string(nil), c.Remote.TrustedProxies...)
	output := options.LogOutput
	if output == nil {
		output = io.Discard
	}
	a := &App{cfg: c, registry: options.Registry, internal: options.InternalAuth, logger: logsafe.New(output), version: options.Version, now: time.Now, resolvers: make(chan struct{}, c.HTTP.ResolverConcurrency), connections: make(chan struct{}, c.HTTP.MaxConnections), limiter: newLimiter(c.HTTP)}
	a.listen = net.Listen
	a.tokens = auth.NewStore(c.Storage.StateDir)
	if _, err := a.tokens.List(); err != nil {
		return nil, errors.New("cannot load valid token state")
	}
	for _, value := range c.LAN.AllowNetworks {
		p := policyPrefix(value)
		a.lanNetworks = append(a.lanNetworks, p)
	}
	for _, value := range c.Remote.TrustedProxies {
		p := policyPrefix(value)
		a.trustedProxies = append(a.trustedProxies, p)
	}
	s, err := store.New(c.Storage.DataDir, c.Storage.StateDir)
	if err != nil {
		return nil, err
	}
	a.catalog = &catalog{store: s, dir: c.Storage.DataDir, stateDir: c.Storage.StateDir, ids: c.Providers.Enabled, cache: make(map[string]cachedSnapshot), metadata: make(map[string]cachedMetadata)}
	if err := a.catalog.reload(true); err != nil {
		if a.catalog.view.Load() == nil {
			return nil, err
		}
		a.logger.Warn("catalog degraded", "code", "catalog_degraded")
	}
	return a, nil
}

// Reload observes completed atomic snapshots. Failed reloads retain the last
// valid in-memory generation and mark health degraded; no persisted state changes.
func (a *App) Reload() error { return a.catalog.reload(false) }

func (a *App) Handler(kind ListenerKind) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.handle(kind, w, r) })
}
func duration(value string) time.Duration { d, _ := time.ParseDuration(value); return d }
