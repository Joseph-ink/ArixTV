// Package builtin wires independently implemented providers into the binary.
// Constructors do no network I/O; enabled snapshots remain the serving catalog.
package builtin

import (
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/provider/vizio"
)

func NewRegistry() (*provider.Registry, error) {
	return provider.NewRegistry(provider.Entry{Metadata: vizio.Metadata(), Runtime: vizio.New()})
}
