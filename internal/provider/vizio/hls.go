package vizio

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Joseph-ink/ArixTV/internal/channel"
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

func inspectManifest(body string) (channel.DRM, string, error) {
	if !utf8.ValidString(body) || strings.ContainsRune(body, '\x00') {
		return "", "", ErrManifest
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "#EXTM3U" {
		return "", "", ErrManifest
	}
	drm := channel.Clear
	variant := ""
	awaiting := false
	master := false
	media := false
	target := false
	segment := false
	awaitingSegment := false
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if awaiting && !strings.HasPrefix(line, "#") {
			if variant == "" {
				variant = line
			}
			awaiting = false
			continue
		}
		if !strings.HasPrefix(line, "#") {
			if !awaitingSegment || !validKeyURI(line) {
				return "", "", ErrManifest
			}
			segment = true
			awaitingSegment = false
			continue
		}
		switch {
		case strings.HasPrefix(line, "#EXT-X-SESSION-KEY:"), strings.HasPrefix(line, "#EXT-X-KEY:"):
			_, value, _ := strings.Cut(line, ":")
			attrs, err := attributes(value)
			if err != nil {
				return "", "", err
			}
			switch attrs["METHOD"] {
			case "NONE":
			case "AES-128":
				if format := attrs["KEYFORMAT"]; format != "" && format != "identity" {
					return "", "", ErrUnavailable
				}
				// Classification only. Never fetch a key, regardless of encryption.
				if !validKeyURI(attrs["URI"]) || needsRelay(attrs["URI"]) {
					return "", "", ErrUnavailable
				}
				drm = channel.HLSStandardEncrypted
			default:
				return "", "", ErrUnavailable // SAMPLE-AES / protected or unknown systems
			}
		case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			attrs, err := attributes(strings.TrimPrefix(line, "#EXT-X-MEDIA:"))
			// Selecting a muxed variant intentionally omits optional subtitles.
			// Separate audio/video would make that variant incomplete.
			if err != nil || (attrs["URI"] != "" && attrs["TYPE"] != "SUBTITLES" && attrs["TYPE"] != "CLOSED-CAPTIONS") {
				return "", "", ErrManifest
			}
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			if awaiting {
				return "", "", ErrManifest
			}
			master = true
			awaiting = true
		case strings.HasPrefix(line, "#EXTINF:"):
			if awaitingSegment {
				return "", "", ErrManifest
			}
			media = true
			awaitingSegment = true
		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			n, err := strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"))
			if err != nil || n <= 0 {
				return "", "", ErrManifest
			}
			target = true
		case strings.HasPrefix(line, "#EXT-X-DEFINE:"):
			return "", "", ErrManifest // no variable substitution
		}
	}
	if awaiting || awaitingSegment || (master && (variant == "" || media)) || (!master && (!media || !target || !segment)) {
		return "", "", ErrManifest
	}
	return drm, variant, nil
}

// Explicit IP binding is relay-only. Absence of such a marker is not proof of
// cross-egress compatibility; experimental live validation must establish that.
func needsRelay(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	for key, values := range u.Query() {
		switch strings.ToLower(key) {
		case "ip", "client_ip", "remote_ip", "request_ip":
			for _, v := range values {
				if v != "" {
					return true
				}
			}
		}
	}
	return false
}

func validKeyURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || raw == "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "{}\\\r\n") {
		return false
	}
	if u.IsAbs() {
		_, err = publicURL(raw)
		return err == nil
	}
	return u.Host == "" && !strings.HasPrefix(raw, "//")
}

// attributes parses the ordinary HLS attribute-list grammar, preserving quoted
// commas and rejecting duplicate/unterminated values rather than guessing.
func attributes(s string) (map[string]string, error) {
	result := map[string]string{}
	for s != "" {
		key, rest, ok := strings.Cut(s, "=")
		if !ok || key == "" {
			return nil, ErrManifest
		}
		for _, r := range key {
			if (r < 'A' || r > 'Z') && r != '-' && (r < '0' || r > '9') {
				return nil, ErrManifest
			}
		}
		if _, exists := result[key]; exists {
			return nil, ErrManifest
		}
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				return nil, ErrManifest
			}
			end++
			value = rest[1:end]
			s = rest[end+1:]
			if s != "" {
				if s[0] != ',' {
					return nil, ErrManifest
				}
				s = s[1:]
				if s == "" {
					return nil, ErrManifest
				}
			}
		} else {
			value, s, _ = strings.Cut(rest, ",")
			if value == "" {
				return nil, ErrManifest
			}
		}
		result[key] = value
	}
	return result, nil
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
