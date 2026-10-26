//go:build integration

package xumo

import (
	"context"
	"os"
	"testing"
	"time"
)

// Tag AND environment opt-in are required. One discovery and one Resolve only;
// at most one JSON pointer and two manifests, no guide/key/segment requests.
func TestLiveXumo(t *testing.T) {
	if os.Getenv("ARIXTV_XUMO_LIVE") != "1" {
		t.Skip("set ARIXTV_XUMO_LIVE=1 to permit live Xumo requests")
	}
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cs, err := p.Discover(ctx)
	if err != nil {
		t.Fatalf("live discovery failed (region/network/protocol dependent): %v", err)
	}
	if len(cs) == 0 {
		t.Fatal("live catalog empty")
	}
	var direct, protected, other int
	for _, c := range cs {
		if c.DirectEligible() {
			direct++
		} else if c.DRM.Protected() {
			protected++
		} else {
			other++
		}
	}
	t.Logf("live discovery: %d channels; %d evidence-qualified Direct; %d DRM-marked; %d other/unknown", len(cs), direct, protected, other)
	for _, c := range cs {
		if c.ProviderChannelID == referenceID {
			if !c.DirectEligible() {
				t.Fatal("reference channel no longer qualifies for Direct")
			}
			s, err := p.Resolve(ctx, c)
			if err != nil {
				t.Fatalf("live Resolve failed (region/network/protocol dependent): %v", err)
			}
			if s.Validate(time.Now()) != nil || s.Capability != "clear_direct" {
				t.Fatal("live result is expired, invalid or relay-only")
			}
			t.Log("fresh source and bounded HLS media manifest validated without special playback headers/cookies; no keys or segments fetched")
			return
		}
	}
	t.Fatal("reference channel absent; no unverified fallback was probed")
}
