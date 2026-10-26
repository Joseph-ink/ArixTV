package builtin

import (
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"testing"
)

func TestOnlyVizioRegisteredWithoutNetwork(t *testing.T) {
	r, e := NewRegistry()
	if e != nil {
		t.Fatal(e)
	}
	if len(r.List()) != 1 {
		t.Fatal("unexpected provider registered")
	}
	m, p, ok := r.Lookup("vizio")
	if !ok || p == nil || m.Status != provider.Experimental || m.Access != provider.AnonymousFree || m.UnofficialMirror {
		t.Fatal("incorrect Vizio registration")
	}
	if e := r.ValidateEnabled([]string{"vizio"}); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"xumo", "plex", "pluto", "lg-channels", "roku", "samsung"} {
		if e := r.ValidateEnabled([]string{id}); e == nil {
			t.Fatal("second provider available")
		}
	}
}
