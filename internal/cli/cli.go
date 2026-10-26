// Package cli implements the staged command surface without process-global flags.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/Joseph-ink/ArixTV/internal/config"
	"github.com/Joseph-ink/ArixTV/internal/provider/builtin"
	"github.com/Joseph-ink/ArixTV/internal/server"
)

const usage = `Usage:
  arixtv version
  arixtv config validate [--config PATH]
  arixtv serve [--config PATH]

The default config path is ./config.toml. Provider/refresh/token management commands remain staged for later phases.
`

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// Run returns 0 on success, 1 on configuration/startup/runtime failure, and 2 on usage errors.
func Run(args []string, stdout, stderr io.Writer, build BuildInfo) int {
	return RunContext(context.Background(), args, stdout, stderr, build)
}

func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer, build BuildInfo) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "arixtv %s\ncommit: %s\nbuild date: %s\n", build.Version, build.Commit, build.BuildDate)
		return 0
	}
	serve := len(args) > 0 && args[0] == "serve"
	if !serve && (len(args) < 2 || args[0] != "config" || args[1] != "validate") {
		fmt.Fprint(stderr, usage)
		return 2
	}
	flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
	// flag errors may contain user input; print our own value-free diagnostics.
	flags.SetOutput(io.Discard)
	path := flags.String("config", "config.toml", "configuration file")
	flagArgs := args[1:]
	if !serve {
		flagArgs = args[2:]
	}
	if err := flags.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprint(stderr, usage)
		return 2
	}
	if flags.NArg() != 0 || *path == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	c, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	if serve {
		registry, err := builtin.NewRegistry()
		if err != nil {
			fmt.Fprintln(stderr, "error: provider registry initialization failed")
			return 1
		}
		app, err := server.New(c, server.Options{Registry: registry, LogOutput: stderr, Version: build.Version})
		if err == nil {
			err = app.Run(ctx)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: %s\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "configuration valid")
	return 0
}
