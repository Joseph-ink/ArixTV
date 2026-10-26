package xumo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

const maxPlaybackJSON = 1 << 20

type epochWindow struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}
type broadcastAsset struct {
	ID         string `json:"id"`
	Live       *bool  `json:"live"`
	Type       string `json:"type"`
	Timestamps struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	} `json:"timestamps"`
}

func (p *Provider) resolveCurrent(ctx context.Context, c channel.Channel) (channel.Stream, error) {
	raw := fmt.Sprintf("%s/v2/channels/channel/%s/broadcast.json?hour=%d", apiOrigin, c.ProviderChannelID, p.now().UTC().Hour())
	res, err := p.get(ctx, raw, "application/json", maxPlaybackJSON)
	if err != nil {
		return channel.Stream{}, err
	}
	root, err := object(res.body, "refresh", "assets")
	if err != nil {
		return channel.Stream{}, err
	}
	var refresh epochWindow
	// The observed API returns future refresh ranges for an already-live asset.
	// This is a polling/freshness hint, not the content's availability start.
	// Retain its future upper bound conservatively and reject exhausted hints.
	if json.Unmarshal(root["refresh"], &refresh) != nil || refresh.From <= 0 || refresh.To <= refresh.From || refresh.To <= p.now().Unix() {
		return channel.Stream{}, ErrProtocol
	}
	assets, err := array[broadcastAsset](root["assets"], 128)
	if err != nil || len(assets) == 0 {
		return channel.Stream{}, ErrProtocol
	}
	var chosen *broadcastAsset
	seen := map[string]bool{}
	for i := range assets {
		a := &assets[i]
		if !assetID(a.ID) || a.Live == nil || a.Type != "Asset" || seen[a.ID] {
			return channel.Stream{}, ErrProtocol
		}
		seen[a.ID] = true
		if !*a.Live {
			continue
		}
		if chosen != nil || !activeWindow(a.Timestamps.Start, a.Timestamps.End, p.now()) {
			return channel.Stream{}, ErrUnavailable
		}
		chosen = a
	}
	if chosen == nil {
		return channel.Stream{}, ErrUnavailable
	}
	s := channel.Stream{StreamType: channel.HLS, DRM: channel.Clear, Capability: c.Capability}
	limitExpiry(&s, time.Unix(refresh.To, 0))
	limitExpiry(&s, time.Unix(chosen.Timestamps.End, 0))
	if res.cookies {
		s.Capability = channel.ClearRelayOnly
	}
	source, until, cookies, err := p.source(ctx, chosen.ID)
	if err != nil {
		return channel.Stream{}, err
	}
	limitExpiry(&s, until)
	if cookies {
		s.Capability = channel.ClearRelayOnly
	}
	return p.resolveHLS(ctx, source, s)
}
func activeWindow(start, end int64, now time.Time) bool {
	return start > 0 && end > start && now.Unix() >= start && now.Unix() < end
}
func assetID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
func (p *Provider) source(ctx context.Context, id string) (string, time.Time, bool, error) {
	// Request only providers; no title, programs, cue points or guide metadata.
	res, err := p.get(ctx, apiOrigin+"/v2/assets/asset/"+id+".json?f=providers", "application/json", maxPlaybackJSON)
	if err != nil {
		return "", time.Time{}, false, err
	}
	root, err := object(res.body, "id", "contentType", "providers", "type")
	if err != nil {
		return "", time.Time{}, false, err
	}
	var got, content, kind string
	if json.Unmarshal(root["id"], &got) != nil || got != id || json.Unmarshal(root["contentType"], &content) != nil || content != "SIMULCAST" || json.Unmarshal(root["type"], &kind) != nil || kind != "Asset" {
		return "", time.Time{}, false, ErrProtocol
	}
	providers, err := array[json.RawMessage](root["providers"], 32)
	if err != nil || len(providers) == 0 {
		return "", time.Time{}, false, ErrProtocol
	}
	for _, entry := range providers {
		fields, err := object(entry, "id", "title", "color", "sources", "sunrise", "sunset")
		if err != nil {
			return "", time.Time{}, false, err
		}
		from, err := providerTime(fields["sunrise"])
		if err != nil {
			return "", time.Time{}, false, err
		}
		to, err := providerTime(fields["sunset"])
		if err != nil || !to.After(from) {
			return "", time.Time{}, false, ErrProtocol
		}
		sources, err := array[json.RawMessage](fields["sources"], 16)
		if err != nil || len(sources) == 0 {
			return "", time.Time{}, false, ErrProtocol
		}
		if p.now().Before(from) || !p.now().Before(to) {
			continue
		}
		for _, raw := range sources {
			src, err := object(raw, "uri", "width", "height", "lang", "hasEmbeddedCaptions", "produces")
			if err != nil {
				return "", time.Time{}, false, err
			}
			var uri, mime string
			if json.Unmarshal(src["uri"], &uri) != nil || uri == "" || json.Unmarshal(src["produces"], &mime) != nil {
				return "", time.Time{}, false, ErrProtocol
			}
			if mime != "application/x-mpegURL" && mime != "application/vnd.apple.mpegurl" {
				continue
			}
			expanded, err := p.expandSource(uri)
			if err != nil {
				return "", time.Time{}, false, err
			}
			return expanded, to, res.cookies, nil
		}
	}
	return "", time.Time{}, false, ErrUnavailable
}
func providerTime(raw []byte) (time.Time, error) {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return time.Time{}, ErrProtocol
	}
	t, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", s)
	if err != nil {
		return time.Time{}, ErrProtocol
	}
	return t.UTC(), nil
}
func limitExpiry(s *channel.Stream, t time.Time) {
	t = t.UTC()
	if s.ExpiresAt == nil || t.Before(*s.ExpiresAt) {
		s.ExpiresAt = &t
	}
}
func urlExpiry(s *channel.Stream, raw string, now time.Time) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrUnavailable
	}
	// Xumo's observed ad-routing values contain a literal semicolon, valid in
	// an opaque URI query but rejected by Go's form-oriented ParseQuery. Inspect
	// only expiry parameters without parsing or reserializing unrelated values.
	seen := map[string]bool{}
	for _, part := range strings.Split(u.RawQuery, "&") {
		key, value, _ := strings.Cut(part, "=")
		key, err = url.QueryUnescape(key)
		if err != nil {
			return ErrUnavailable
		}
		if key != "exp" && key != "expires" {
			continue
		}
		if seen[key] {
			return ErrUnavailable
		}
		seen[key] = true
		value, err = url.QueryUnescape(value)
		if err != nil {
			return ErrUnavailable
		}
		seconds, err := strconv.ParseInt(value, 10, 64)
		if err != nil || seconds <= now.Unix() {
			return ErrUnavailable
		}
		limitExpiry(s, time.Unix(seconds, 0))
	}
	return nil
}

// Only independently observed public query placeholders are supported. Untouched
// signed values remain byte-for-byte intact; unknown identity protocols fail closed.
func (p *Provider) expandSource(raw string) (string, error) {
	u, err := publicURL(raw)
	if err != nil {
		return "", ErrUnavailable
	}
	if strings.ContainsAny(u.Path, "[]{}") {
		return "", ErrUnavailable
	}
	parts := strings.Split(u.RawQuery, "&")
	for i, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return "", ErrUnavailable
		}
		if !strings.ContainsAny(decoded, "[]{}") {
			continue
		}
		for strings.Contains(decoded, "[") {
			start := strings.IndexByte(decoded, '[')
			end := strings.IndexByte(decoded[start:], ']')
			if end < 0 {
				return "", ErrUnavailable
			}
			end += start
			var replacement string
			switch decoded[start+1 : end] {
			case "deviceID":
				replacement = p.deviceID
			case "PPID":
				replacement = p.publisherID
			case "LIMITED_AD_TRACKING":
				replacement = "1"
			case "app_bundle":
				replacement = "org.arixtv"
			default:
				return "", ErrUnavailable
			}
			decoded = decoded[:start] + replacement + decoded[end+1:]
		}
		if strings.ContainsAny(decoded, "]{}") {
			return "", ErrUnavailable
		}
		parts[i] = key + "=" + url.QueryEscape(decoded)
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String(), nil
}
