// Package builtin wires independently implemented providers into the binary.
// Constructors do no network I/O; enabled snapshots remain the serving catalog.
package builtin

import (
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/provider/vizio"
	"github.com/Joseph-ink/ArixTV/internal/provider/xumo"
)

func NewRegistry() (*provider.Registry, error) {
	x, err := xumo.New()
	if err != nil {
		return nil, err
	}
	return provider.NewRegistry(provider.Entry{Metadata: vizio.Metadata(), Runtime: vizio.New()}, provider.Entry{Metadata: xumo.Metadata(), Runtime: x})
}
