//go:build integration

package vizio

import (
	"context"
	"os"
	"testing"
	"time"
)

// Explicit tag AND environment opt-in prevent accidental live-provider CI work.
// One catalog discovery and one fresh Resolve (catalog + <=2 manifests), no media.
func TestLiveVizio(t *testing.T) {
	if os.Getenv("ARIXTV_VIZIO_LIVE") != "1" {
		t.Skip("set ARIXTV_VIZIO_LIVE=1 to permit live provider requests")
	}
	p := New()
	defer p.client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cs, e := p.Discover(ctx)
	if e != nil {
		t.Fatalf("live discovery failed (region/network/upstream dependent): %v", e)
	}
	if len(cs) == 0 {
		t.Fatal("live catalog empty")
	}
	var direct, protected, unknown int
	for _, c := range cs {
		if c.DirectEligible() {
			direct++
		} else if c.DRM.Protected() {
			protected++
		} else {
			unknown++
		}
	}
	t.Logf("live catalog: %d channels; %d catalog-qualified Direct candidates; %d protected; %d other/unknown", len(cs), direct, protected, unknown)
	id := os.Getenv("ARIXTV_VIZIO_CHANNEL")
	if id == "" {
		id = "98wurl_bloomberg_quicktake_3"
	}
	for _, c := range cs {
		if c.ProviderChannelID == id {
			s, e := p.Resolve(ctx, c)
			if e != nil {
				t.Fatalf("live Resolve failed (region/network/upstream dependent): %v", e)
			}
			if e := s.Validate(time.Now()); e != nil {
				t.Fatalf("invalid live stream: %v", e)
			}
			if s.Capability != "clear_direct" {
				t.Fatal("live stream requires relay or is unsupported")
			}
			t.Log("fresh HLS media manifest reached and validated; no keys or segments fetched")
			return
		}
	}
	t.Fatal("chosen live channel absent; select ARIXTV_VIZIO_CHANNEL explicitly")
}
