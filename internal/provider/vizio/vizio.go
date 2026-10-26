// Package vizio independently implements anonymous WatchFree+ catalog discovery
// and tune-time resolution. It never requests guide data, keys or media segments.
package vizio

import (
	"context"
	"net/http"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/provider"
)

const catalogURL = "https://watchfreeplus-epg-prod.smartcasttv.com/api/channels"

type Provider struct {
	client   *http.Client
	endpoint string
	now      func() time.Time
	active   chan struct{}
}

var _ provider.Provider = (*Provider)(nil)

// New creates one reusable client/transport, no sessions, workers or network I/O.
func New() *Provider {
	return &Provider{client: newClient(), endpoint: catalogURL, now: time.Now, active: make(chan struct{}, 2)}
}

func Metadata() provider.Metadata {
	return provider.Metadata{ID: "vizio", Name: "Vizio WatchFree+", Status: provider.Experimental, Access: provider.AnonymousFree,
		Capabilities: []channel.Capability{channel.ClearDirect, channel.ClearRelayOnly, channel.DRMPlayback, channel.Unsupported, channel.UnknownCapability}}
}

func (p *Provider) begin(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	select {
	case p.active <- struct{}{}:
	default:
		return nil, nil, ErrBusy
	}
	c, cancel := context.WithTimeout(ctx, operationTimeout)
	return c, func() { cancel(); <-p.active }, nil
}

// Discover returns nil on any incomplete/invalid catalog, never a partial slice.
func (p *Provider) Discover(ctx context.Context) ([]channel.Channel, error) {
	ctx, done, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	rows, err := p.catalog(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]channel.Channel, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.channel)
	}
	return result, nil
}

// Resolve refetches the catalog; persisted metadata is never a playback source.
// One master and one muxed media manifest may be inspected, never keys/segments.
func (p *Provider) Resolve(ctx context.Context, ch channel.Channel) (channel.Stream, error) {
	if !resolvable(ch) {
		return channel.Stream{}, ErrUnavailable
	}
	ctx, done, err := p.begin(ctx)
	if err != nil {
		return channel.Stream{}, err
	}
	defer done()
	rows, err := p.catalog(ctx)
	if err != nil {
		return channel.Stream{}, err
	}
	for _, row := range rows {
		if row.channel.ProviderChannelID != ch.ProviderChannelID {
			continue
		}
		if !resolvable(row.channel) {
			return channel.Stream{}, ErrUnavailable
		}
		return p.resolveHLS(ctx, row.playback)
	}
	return channel.Stream{}, ErrUnavailable
}

func resolvable(ch channel.Channel) bool {
	return ch.ProviderID == "vizio" && ch.Validate() == nil && (ch.Capability == channel.ClearDirect || ch.Capability == channel.ClearRelayOnly)
}
