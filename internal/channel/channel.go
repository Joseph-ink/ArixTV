package channel

import (
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Channel is a small persisted identity. No resolved URL, headers, session,
// arbitrary provider payload or account credentials belong in this structure.
// Store schema changes require a snapshot version/migration decision.
type Channel struct {
	ProviderID        string     `json:"provider_id"`
	ProviderChannelID string     `json:"provider_channel_id"`
	Name              string     `json:"name"`
	LogoURL           string     `json:"logo_url,omitempty"`
	Group             string     `json:"group,omitempty"`
	Country           string     `json:"country,omitempty"`
	Language          string     `json:"language,omitempty"`
	StreamType        StreamType `json:"stream_type"`
	DRM               DRM        `json:"drm"`
	Capability        Capability `json:"playback_capability"`
	// Fingerprint is an optional SHA-256 digest of meaningful provider metadata.
	Fingerprint string    `json:"fingerprint,omitempty"`
	LastSeen    time.Time `json:"last_seen"`
}

func ValidProviderID(id string) bool {
	if len(id) == 0 || len(id) > 64 || id[0] < 'a' || id[0] > 'z' {
		return false
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func validText(s string, max int, required bool) bool {
	if !utf8.ValidString(s) || len(s) > max || (required && strings.TrimSpace(s) == "") {
		return false
	}
	return !strings.ContainsFunc(s, unicode.IsControl)
}

func (c Channel) Validate() error {
	if !ValidProviderID(c.ProviderID) || !validText(c.ProviderChannelID, 256, true) || !validText(c.Name, 512, true) {
		return errors.New("invalid channel identity")
	}
	if !validText(c.Group, 256, false) || !validText(c.Country, 8, false) || !validText(c.Language, 64, false) {
		return errors.New("invalid channel metadata")
	}
	if c.LogoURL != "" {
		u, err := url.Parse(c.LogoURL)
		// Snapshot artwork must be public and stable, without signed query values.
		if err != nil || !HTTPURL(u) || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || len(c.LogoURL) > 2048 {
			return errors.New("logo must be a public HTTP URL without credentials, query or fragment")
		}
	}
	if c.Fingerprint != "" {
		digest, err := hex.DecodeString(c.Fingerprint)
		if err != nil || len(digest) != 32 {
			return errors.New("invalid metadata fingerprint")
		}
	}
	if c.LastSeen.IsZero() {
		return errors.New("channel last_seen is required")
	}
	return ValidateClassification(c.StreamType, c.DRM, c.Capability)
}

func HTTPURL(u *url.URL) bool {
	return u != nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.Opaque == ""
}

// DirectEligible is the common policy gate for future LAN/remote playlists.
func (c Channel) DirectEligible() bool {
	return c.Validate() == nil && c.Capability == ClearDirect
}
