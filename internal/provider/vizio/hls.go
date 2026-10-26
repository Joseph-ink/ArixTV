package vizio

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/provider/hlsinspect"
)

const maxManifestBytes = 256 << 10

// The small inspector deliberately supports a bounded subset: a media manifest
// or one master -> muxed media variant. Separate audio/video tracks and deeper
// indirection fail closed. It never rewrites manifests or retrieves keys/media.
func (p *Provider) resolveHLS(ctx context.Context, raw string) (channel.Stream, error) {
	s := channel.Stream{StreamType: channel.HLS, DRM: channel.Clear, Capability: channel.ClearDirect}
	for depth := 0; depth < 2; depth++ {
		if _, err := publicURL(raw); err != nil {
			return channel.Stream{}, ErrUnavailable
		}
		if err := expiry(&s, raw, p.now()); err != nil {
			return channel.Stream{}, err
		}
		if needsRelay(raw) {
			s.Capability = channel.ClearRelayOnly
		}
		res, err := p.get(ctx, raw, "application/vnd.apple.mpegurl, application/x-mpegURL", maxManifestBytes)
		if err != nil {
			return channel.Stream{}, err
		}
		if _, err := publicURL(res.url.String()); err != nil {
			return channel.Stream{}, ErrUnavailable
		}
		if err := expiry(&s, res.url.String(), p.now()); err != nil {
			return channel.Stream{}, err
		}
		if res.cookies || needsRelay(res.url.String()) {
			s.Capability = channel.ClearRelayOnly
		}
		drm, variant, err := inspectManifest(string(res.body))
		if err != nil {
			return channel.Stream{}, err
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
		u, err := url.Parse(variant)
		if err != nil {
			return channel.Stream{}, ErrManifest
		}
		raw = res.url.ResolveReference(u).String()
	}
	return channel.Stream{}, ErrManifest
}

// Thin adapters preserve Vizio's existing error identities and URI policy.
func inspectManifest(body string) (channel.DRM, string, error) {
	drm, variant, err := hlsinspect.Inspect(body, absoluteURI)
	return drm, variant, inspectionError(err)
}
func needsRelay(raw string) bool  { return hlsinspect.NeedsRelay(raw) }
func validKeyURI(raw string) bool { return hlsinspect.ValidURI(raw, absoluteURI) }
func attributes(raw string) (map[string]string, error) {
	values, err := hlsinspect.Attributes(raw)
	return values, inspectionError(err)
}
func absoluteURI(raw string) bool { _, err := publicURL(raw); return err == nil }
func inspectionError(err error) error {
	switch err {
	case nil:
		return nil
	case hlsinspect.ErrProtection:
		return ErrUnavailable
	default:
		return ErrManifest
	}
}

func expiry(s *channel.Stream, raw string, now time.Time) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrUnavailable
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return ErrUnavailable
	}
	for _, key := range []string{"exp", "expires"} {
		v, ok := values[key]
		if !ok {
			continue
		}
		if len(v) != 1 {
			return ErrUnavailable
		}
		n, err := strconv.ParseInt(v[0], 10, 64)
		if err != nil || n <= 0 {
			return ErrUnavailable
		}
		t := time.Unix(n, 0).UTC()
		if !t.After(now) {
			return ErrUnavailable
		}
		if s.ExpiresAt == nil || t.Before(*s.ExpiresAt) {
			s.ExpiresAt = &t
		}
	}
	return nil
}
