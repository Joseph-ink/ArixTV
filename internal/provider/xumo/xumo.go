// Package xumo independently implements anonymous Xumo catalog discovery and
// current-playback resolution. It does not fetch guide data or media segments.
package xumo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/provider"
)

const apiOrigin = "https://valencia-app-mds.xumo.com"
const catalogPath = "/v2/proxy/channels/list/10006.json?sort=hybrid"

// Provider owns ephemeral anonymous identifiers and one reused client. It starts
// no workers and has no mutable catalog/session cache or persistent cookie jar.
type Provider struct {
	client      *http.Client
	now         func() time.Time
	active      chan struct{}
	deviceID    string
	publisherID string
}

var _ provider.Provider = (*Provider)(nil)

func New() (*Provider, error) {
	var identity [32]byte
	if _, err := rand.Read(identity[:]); err != nil {
		return nil, ErrIdentity
	}
	return &Provider{client: newClient(), now: time.Now, active: make(chan struct{}, 2), deviceID: hex.EncodeToString(identity[:16]), publisherID: hex.EncodeToString(identity[16:])}, nil
}
func (*Provider) String() string       { return "[xumo provider]" }
func (*Provider) GoString() string     { return "[xumo provider]" }
func (*Provider) LogValue() slog.Value { return slog.StringValue("[xumo provider]") }
func Metadata() provider.Metadata {
	return provider.Metadata{ID: "xumo", Name: "Xumo Play", Status: provider.Experimental, Access: provider.AnonymousFree, Capabilities: []channel.Capability{channel.ClearDirect, channel.ClearRelayOnly, channel.DRMPlayback, channel.Unsupported, channel.UnknownCapability}}
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
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	return ctx, func() { cancel(); <-p.active }, nil
}

// Discover returns nil/error for any incomplete candidate. Routine discovery
// does exactly one catalog request, with no channel/manifest/guide probes.
func (p *Provider) Discover(ctx context.Context) ([]channel.Channel, error) {
	ctx, done, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	channels, err := p.catalog(ctx)
	if err != nil {
		return nil, err
	}
	return channels, nil
}
func resolvable(c channel.Channel) bool {
	return c.ProviderID == "xumo" && c.Validate() == nil && (c.Capability == channel.ClearDirect || c.Capability == channel.ClearRelayOnly)
}
func (p *Provider) Resolve(ctx context.Context, ch channel.Channel) (channel.Stream, error) {
	if !resolvable(ch) {
		return channel.Stream{}, ErrUnavailable
	}
	ctx, done, err := p.begin(ctx)
	if err != nil {
		return channel.Stream{}, err
	}
	defer done()
	current, err := p.catalog(ctx)
	if err != nil {
		return channel.Stream{}, err
	}
	for _, c := range current {
		if c.ProviderChannelID != ch.ProviderChannelID {
			continue
		}
		if !resolvable(c) {
			return channel.Stream{}, ErrUnavailable
		}
		return p.resolveCurrent(ctx, c)
	}
	return channel.Stream{}, ErrUnavailable
}
