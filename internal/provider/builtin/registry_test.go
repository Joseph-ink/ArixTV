package builtin

import (
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"testing"
)

func TestOnlyVizioAndXumoRegisteredWithoutNetwork(t *testing.T) {
	r, e := NewRegistry()
	if e != nil {
		t.Fatal(e)
	}
	if len(r.List()) != 2 || r.List()[0].ID != "vizio" || r.List()[1].ID != "xumo" {
		t.Fatal("unexpected provider registered")
	}
	for _, id := range []string{"vizio", "xumo"} {
		m, p, ok := r.Lookup(id)
		if !ok || p == nil || m.Status != provider.Experimental || m.Access != provider.AnonymousFree || m.UnofficialMirror {
			t.Fatal("incorrect provider registration")
		}
	}
	if e := r.ValidateEnabled([]string{"vizio", "xumo"}); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"plex", "pluto", "lg-channels", "roku", "samsung"} {
		if e := r.ValidateEnabled([]string{id}); e == nil {
			t.Fatal("unapproved provider available")
		}
	}
}
