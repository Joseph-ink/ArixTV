// Package hlsinspect contains a small, pure HLS classification inspector.
// It does no networking, key retrieval, manifest rewriting or media handling.
// Callers supply their absolute-URL policy and retain their own fetch budgets.
package hlsinspect

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

var (
	ErrManifest   = errors.New("unsupported or invalid HLS manifest")
	ErrProtection = errors.New("protected or unsupported HLS key delivery")
)

// Inspect selects the first muxed variant or classifies a media manifest.
// Optional subtitles are omitted; separate audio/video and deeper nesting are
// handled conservatively by the caller. publicURL validates absolute references.
func Inspect(body string, publicURL func(string) bool) (channel.DRM, string, error) {
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
			if !awaitingSegment || !ValidURI(line, publicURL) {
				return "", "", ErrManifest
			}
			segment = true
			awaitingSegment = false
			continue
		}
		switch {
		case strings.HasPrefix(line, "#EXT-X-SESSION-KEY:"), strings.HasPrefix(line, "#EXT-X-KEY:"):
			_, value, _ := strings.Cut(line, ":")
			attrs, err := Attributes(value)
			if err != nil {
				return "", "", err
			}
			switch attrs["METHOD"] {
			case "NONE":
			case "AES-128":
				if format := attrs["KEYFORMAT"]; format != "" && format != "identity" {
					return "", "", ErrProtection
				}
				// Classification only. Never fetch a key, regardless of encryption.
				if !ValidURI(attrs["URI"], publicURL) || NeedsRelay(attrs["URI"]) {
					return "", "", ErrProtection
				}
				drm = channel.HLSStandardEncrypted
			default:
				return "", "", ErrProtection // SAMPLE-AES / protected or unknown systems
			}
		case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			attrs, err := Attributes(strings.TrimPrefix(line, "#EXT-X-MEDIA:"))
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
func NeedsRelay(raw string) bool {
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

func ValidURI(raw string, publicURL func(string) bool) bool {
	u, err := url.Parse(raw)
	if err != nil || raw == "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "{}\\\r\n") {
		return false
	}
	if u.IsAbs() {
		return publicURL(raw)
	}
	return u.Host == "" && !strings.HasPrefix(raw, "//")
}

// attributes parses the ordinary HLS attribute-list grammar, preserving quoted
// commas and rejecting duplicate/unterminated values rather than guessing.
func Attributes(s string) (map[string]string, error) {
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
