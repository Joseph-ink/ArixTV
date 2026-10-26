package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Joseph-ink/ArixTV/internal/cli"
)

// Populated with -ldflags at build time. Development builds keep these defaults.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.RunContext(ctx, os.Args[1:], os.Stdout, os.Stderr, cli.BuildInfo{
		Version: version, Commit: commit, BuildDate: buildDate,
	})
	stop()
	os.Exit(code)
}
