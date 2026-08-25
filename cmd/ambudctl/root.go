// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/runtime"
)

// newRootCmd builds the ambudctl command tree. Each subcommand takes a
// runtimeFactory rather than calling runtime.New directly, so tests can
// substitute a runtime.Fake instead of needing a live containerd socket
// — see docs/ROADMAP.md's Phase 1 note on keeping runtime-dependent
// logic testable without a live daemon.
func newRootCmd() *cobra.Command {
	var socketPath string

	root := &cobra.Command{
		Use:   "ambudctl",
		Short: "ambudctl is the Ambud command-line client",
		Long: `ambudctl is the Ambud command-line client.

At this stage of the project (Phase 1 of docs/ROADMAP.md) it talks
directly to a local containerd socket. There is no node agent or
control plane yet — those arrive in Phase 2 and Phase 3, at which point
ambudctl starts talking to them over HTTP instead.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&socketPath, "socket", runtime.DefaultSocketPath,
		"containerd socket path")

	factory := func() (runtime.Runtime, error) {
		return runtime.New(socketPath)
	}

	root.AddCommand(newRunCmd(factory))
	root.AddCommand(newPSCmd(factory))
	root.AddCommand(newStopCmd(factory))
	root.AddCommand(newVersionCmd())

	return root
}

// runtimeFactory produces a Runtime for a single command invocation.
// The production factory (above) connects to a real containerd socket;
// tests inject one that returns a runtime.Fake.
type runtimeFactory func() (runtime.Runtime, error)
