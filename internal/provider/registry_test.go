package provider

import (
	"context"
	"testing"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

type mockProvider struct{}

func (*mockProvider) Discover(context.Context) ([]channel.Channel, error) { return nil, nil }
func (*mockProvider) Resolve(context.Context, channel.Channel) (channel.Stream, error) {
	return channel.Stream{}, nil
}
func metadata(id string) Metadata {
	return Metadata{ID: id, Name: "Example", Status: Experimental, Access: AnonymousFree, Capabilities: []channel.Capability{channel.ClearDirect}}
}

func TestRegistry(t *testing.T) {
	m := metadata("example")
	r, err := NewRegistry(Entry{m, &mockProvider{}}, Entry{metadata("another"), nil})
	if err != nil {
		t.Fatal(err)
	}
	m.Capabilities[0] = channel.DRMPlayback
	listed := r.List()
	if listed[0].ID != "another" || listed[1].Capabilities[0] != channel.ClearDirect {
		t.Fatal("registry order/ownership")
	}
	listed[1].Capabilities[0] = channel.Unsupported
	got, p, ok := r.Lookup("example")
	if !ok || p == nil || got.Capabilities[0] != channel.ClearDirect {
		t.Fatal("lookup leaked mutation")
	}
	if _, _, ok := r.Lookup("missing"); ok {
		t.Fatal("unknown lookup succeeded")
	}
	if err := r.ValidateEnabled([]string{"example"}); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]string{{"missing"}, {"another"}, {"example", "example"}} {
		if r.ValidateEnabled(ids) == nil {
			t.Fatal("enabled invalid registry entry")
		}
	}
	if _, err := NewRegistry(Entry{metadata("example"), nil}, Entry{metadata("example"), nil}); err == nil {
		t.Fatal("duplicate ID accepted")
	}
	var nilRuntime *mockProvider
	if _, err := NewRegistry(Entry{metadata("example"), nilRuntime}); err == nil {
		t.Fatal("typed nil accepted")
	}
}

func TestMetadataPolicy(t *testing.T) {
	for _, s := range []Status{Stable, Experimental, Disabled, Unsupported} {
		if !s.Valid() {
			t.Fatal(s)
		}
	}
	for _, a := range []Access{AnonymousFree, FreeAccount, PaidAccount, TVEverywhere, UnknownAccess} {
		if !a.Valid() {
			t.Fatal(a)
		}
	}
	if Status("").Valid() || Access("").Valid() {
		t.Fatal("zero enum accepted")
	}
	for _, access := range []Access{FreeAccount, PaidAccount, TVEverywhere, UnknownAccess} {
		m := metadata("example")
		m.Access = access
		r, err := NewRegistry(Entry{m, &mockProvider{}})
		if err != nil {
			t.Fatal(err)
		}
		if r.ValidateEnabled([]string{"example"}) == nil {
			t.Fatal("account provider entered V1")
		}
		m.Status = Stable
		if m.Validate() == nil {
			t.Fatal("nonanonymous stable provider")
		}
	}
	m := metadata("example")
	m.UnofficialMirror = true
	m.Status = Stable
	if m.Validate() == nil {
		t.Fatal("mirror stable by default")
	}
	m.Status = Experimental
	if m.Validate() != nil {
		t.Fatal("experimental mirror not representable")
	}
	m.Capabilities = []channel.Capability{"bad"}
	if m.Validate() == nil {
		t.Fatal("invalid capability accepted")
	}
	m = metadata("../example")
	if m.Validate() == nil {
		t.Fatal("bad provider ID accepted")
	}
}
