package channel

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func example() Channel {
	return Channel{ProviderID: "example", ProviderChannelID: "channel/1", Name: "Example", LogoURL: "https://example.com/logo.png", Group: "News", Country: "US", Language: "en", StreamType: HLS, DRM: Clear, Capability: ClearDirect, Fingerprint: strings.Repeat("a", 64), LastSeen: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)}
}

func TestChannelSerialization(t *testing.T) {
	c := example()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var round Channel
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, round) {
		t.Fatalf("round trip: %+v", round)
	}
	var fields map[string]any
	json.Unmarshal(b, &fields)
	for _, key := range []string{"provider_id", "provider_channel_id", "name", "logo_url", "group", "country", "language", "stream_type", "drm", "playback_capability", "fingerprint", "last_seen"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("missing schema field %s", key)
		}
	}
	if _, ok := fields["url"]; ok {
		t.Fatal("playback URL in channel schema")
	}
}

func TestEnumsAndPolicy(t *testing.T) {
	for _, v := range []StreamType{HLS, DASH, Other, UnknownStream} {
		if !v.Valid() {
			t.Fatal(v)
		}
	}
	for _, v := range []DRM{Clear, HLSStandardEncrypted, Widevine, PlayReady, FairPlay, OtherDRM, UnknownDRM} {
		if !v.Valid() {
			t.Fatal(v)
		}
	}
	for _, v := range []Capability{ClearDirect, ClearRelayOnly, DRMPlayback, Unsupported, UnknownCapability} {
		if !v.Valid() {
			t.Fatal(v)
		}
	}
	if StreamType("").Valid() || DRM("aes").Valid() || Capability("direct").Valid() {
		t.Fatal("accepted undefined enum")
	}
	for _, tc := range []struct {
		drm    DRM
		cap    Capability
		stream StreamType
		valid  bool
		direct bool
	}{
		{Clear, ClearDirect, HLS, true, true},
		{HLSStandardEncrypted, ClearDirect, HLS, true, true},
		{HLSStandardEncrypted, ClearRelayOnly, HLS, true, false},
		{Widevine, DRMPlayback, DASH, true, false},
		{PlayReady, Unsupported, DASH, true, false},
		{FairPlay, ClearRelayOnly, HLS, false, false},
		{OtherDRM, ClearDirect, HLS, false, false},
		{UnknownDRM, ClearDirect, HLS, false, false},
		{Clear, DRMPlayback, HLS, false, false},
		{HLSStandardEncrypted, ClearDirect, DASH, false, false},
		{Clear, ClearDirect, UnknownStream, false, false},
		{UnknownDRM, UnknownCapability, UnknownStream, true, false},
	} {
		c := example()
		c.DRM = tc.drm
		c.Capability = tc.cap
		c.StreamType = tc.stream
		if (c.Validate() == nil) != tc.valid || c.DirectEligible() != tc.direct {
			t.Errorf("policy mismatch: %+v", tc)
		}
	}
}

func TestInvalidChannel(t *testing.T) {
	for name, mutate := range map[string]func(*Channel){
		"path traversal":  func(c *Channel) { c.ProviderID = "../other" },
		"empty channel":   func(c *Channel) { c.ProviderChannelID = "" },
		"invalid UTF8":    func(c *Channel) { c.ProviderChannelID = "\xff" },
		"newline name":    func(c *Channel) { c.Name = "bad\nname" },
		"large metadata":  func(c *Channel) { c.Group = strings.Repeat("a", 257) },
		"signed artwork":  func(c *Channel) { c.LogoURL = "https://example.com/logo?token=PLACEHOLDER" },
		"secret artwork":  func(c *Channel) { c.LogoURL = "https://user:PLACEHOLDER@example.com/logo" },
		"unknown enum":    func(c *Channel) { c.Capability = "invalid" },
		"no timestamp":    func(c *Channel) { c.LastSeen = time.Time{} },
		"bad fingerprint": func(c *Channel) { c.Fingerprint = "opaque provider payload" },
	} {
		t.Run(name, func(t *testing.T) {
			c := example()
			mutate(&c)
			if c.Validate() == nil {
				t.Fatal("accepted invalid channel")
			}
		})
	}
}

func TestEphemeralStream(t *testing.T) {
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	expiry := now.Add(time.Minute)
	s := Stream{URL: "https://example.com/live?token=PLACEHOLDER", StreamType: HLS, DRM: Clear, Capability: ClearDirect, ExpiresAt: &expiry}
	if err := s.Validate(now); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s)
	for _, text := range []string{string(b), fmt.Sprint(s), fmt.Sprintf("%#v", s)} {
		if strings.Contains(text, "PLACEHOLDER") {
			t.Fatal("resolved URL serialized or formatted")
		}
	}
	if s.Validate(expiry) == nil {
		t.Fatal("accepted expired stream")
	}
	s.ExpiresAt = nil
	s.URL = "file:///private/example"
	if s.Validate(now) == nil {
		t.Fatal("accepted non-HTTP stream")
	}
}
