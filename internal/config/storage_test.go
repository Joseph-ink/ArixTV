package config

import (
	"strings"
	"testing"

	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/store"
)

func TestStorageIntegration(t *testing.T) {
	c, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if c.Storage.DataDir != "./data" || c.Storage.StateDir != "./state" {
		t.Fatal("old configs lost path defaults")
	}
	c, err = Parse(strings.NewReader("[storage]\ndata_dir='./custom-data'\nstate_dir='./custom-state'"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.New(c.Storage.DataDir, c.Storage.StateDir); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"[storage]\ndata_dir=''", "[storage]\nstate_dir=' '", "[storage]\nsecrets='PRIVATE_PLACEHOLDER'"} {
		if _, err := Parse(strings.NewReader(bad)); err == nil {
			t.Fatal("invalid storage config accepted")
		}
	}
	r, err := provider.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ValidateProviders(r); err != nil {
		t.Fatal(err)
	}
	c.Providers.Enabled = []string{"example"}
	if err := c.ValidateProviders(r); err == nil {
		t.Fatal("operational provider check accepted missing runtime")
	}
	if c.ValidateProviders(nil) == nil {
		t.Fatal("nil registry accepted")
	}
}
