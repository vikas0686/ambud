// SPDX-License-Identifier: Apache-2.0

// Command ambudctl is the Ambud command-line client.
//
// At this stage of the project (Phase 0 of docs/ROADMAP.md) it does not
// yet talk to an agent or control plane — it exists to prove the module
// builds, tests, and lints cleanly in CI, and to give --version a home.
// Real subcommands (run, ps, stop, ...) land in Phase 1.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/vikas0686/ambud/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		// Nothing more useful to do if writing the error itself fails;
		// the non-zero exit code below is the real signal.
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run contains ambudctl's logic, separated from main so it can be
// exercised by tests without invoking the compiled binary or touching
// os.Exit.
func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("ambudctl", flag.ContinueOnError)
	fs.SetOutput(out)
	showVersion := fs.Bool("version", false, "print the ambudctl version and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		_, err := fmt.Fprintln(out, version.String())
		return err
	}

	_, err := fmt.Fprintln(out, "ambudctl: no commands implemented yet (see docs/ROADMAP.md, Phase 1).")
	return err
}
