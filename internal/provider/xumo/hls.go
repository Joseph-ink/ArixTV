package xumo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/url"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/provider/hlsinspect"
)

const maxManifestBytes = 256 << 10
const maxPointerBytes = 8192

// A bounded optional JSON-pointer hop precedes at most a master and one muxed
// media manifest. Every hop shares the operation context, transport and safety
// policy. Nothing here retrieves a key, segment or alternate audio rendition.
func (p *Provider) resolveHLS(ctx context.Context, raw string, s channel.Stream) (channel.Stream, error) {
	pointer := false
	manifests := 0
	for requests := 0; requests < 3; requests++ {
		if err := urlExpiry(&s, raw, p.now()); err != nil {
			return channel.Stream{}, err
		}
		if hlsinspect.NeedsRelay(raw) {
			s.Capability = channel.ClearRelayOnly
		}
		res, err := p.get(ctx, raw, "application/vnd.apple.mpegurl, application/x-mpegURL, application/json", maxManifestBytes)
		if err != nil {
			return channel.Stream{}, err
		}
		if res.cookies || hlsinspect.NeedsRelay(res.url.String()) {
			s.Capability = channel.ClearRelayOnly
		}
		if err := urlExpiry(&s, res.url.String(), p.now()); err != nil {
			return channel.Stream{}, err
		}
		mediaType, _, _ := mime.ParseMediaType(res.contentType)
		if mediaType == "application/json" {
			if pointer || manifests > 0 || len(res.body) > maxPointerBytes {
				return channel.Stream{}, ErrProtocol
			}
			d := json.NewDecoder(bytes.NewReader(res.body))
			var next string
			if d.Decode(&next) != nil || d.Decode(new(any)) != io.EOF {
				return channel.Stream{}, ErrProtocol
			}
			if _, err := publicURL(next); err != nil {
				return channel.Stream{}, ErrUnavailable
			}
			raw = next
			pointer = true
			continue
		}
		manifests++
		if manifests > 2 {
			return channel.Stream{}, ErrProtocol
		}
		drm, variant, err := hlsinspect.Inspect(string(res.body), func(raw string) bool { _, err := publicURL(raw); return err == nil })
		if err != nil {
			if err == hlsinspect.ErrProtection {
				return channel.Stream{}, ErrUnavailable
			}
			return channel.Stream{}, ErrProtocol
		}
		if drm == channel.HLSStandardEncrypted {
			s.DRM = drm
		}
		if variant == "" {
			s.URL = res.url.String()
			if s.Validate(p.now()) != nil {
				return channel.Stream{}, ErrUnavailable
			}
			return s, nil
		}
		if manifests == 2 {
			return channel.Stream{}, ErrProtocol
		}
		u, err := url.Parse(variant)
		if err != nil {
			return channel.Stream{}, ErrProtocol
		}
		raw = res.url.ResolveReference(u).String()
	}
	return channel.Stream{}, ErrProtocol
}
