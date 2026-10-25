// Package cli implements the Phase 0 command surface without process-global flags.
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/Joseph-ink/ArixTV/internal/config"
)

const usage = `Usage:
  arixtv version
  arixtv config validate [--config PATH]

The default config path is ./config.toml. Later-phase commands are not available.
`

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// Run returns 0 on success, 1 on configuration failure, and 2 on usage errors.
func Run(args []string, stdout, stderr io.Writer, build BuildInfo) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "arixtv %s\ncommit: %s\nbuild date: %s\n", build.Version, build.Commit, build.BuildDate)
		return 0
	}
	if len(args) < 2 || args[0] != "config" || args[1] != "validate" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
	// flag errors may contain user input; print our own value-free diagnostics.
	flags.SetOutput(io.Discard)
	path := flags.String("config", "config.toml", "configuration file")
	if err := flags.Parse(args[2:]); err != nil {
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
	if _, err := config.Load(*path); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "configuration valid")
	return 0
}
