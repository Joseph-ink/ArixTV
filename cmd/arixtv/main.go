package main

import (
	"os"

	"github.com/Joseph-ink/ArixTV/internal/cli"
)

// Populated with -ldflags at build time. Development builds keep these defaults.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, cli.BuildInfo{
		Version: version, Commit: commit, BuildDate: buildDate,
	}))
}
