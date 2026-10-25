package channel

import (
	"errors"
	"log/slog"
	"net/url"
	"time"
)

// Stream is ephemeral. URL is deliberately omitted from JSON and all normal
// string/log representations; future resolvers use the field only at tune time.
type Stream struct {
	URL        string     `json:"-"`
	StreamType StreamType `json:"stream_type"`
	DRM        DRM        `json:"drm"`
	Capability Capability `json:"playback_capability"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

func (s Stream) Validate(now time.Time) error {
	u, err := url.Parse(s.URL)
	if err != nil || !HTTPURL(u) || u.User != nil || u.Fragment != "" {
		return errors.New("invalid resolved HTTP URL")
	}
	if s.ExpiresAt != nil && !s.ExpiresAt.After(now) {
		return errors.New("resolved stream expired")
	}
	return ValidateClassification(s.StreamType, s.DRM, s.Capability)
}

func (Stream) String() string       { return "[resolved stream]" }
func (Stream) GoString() string     { return "[resolved stream]" }
func (Stream) LogValue() slog.Value { return slog.StringValue("[resolved stream]") }
