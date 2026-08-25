// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/apiclient"
	"github.com/vikas0686/ambud/internal/runtime"
)

// defaultAgentURL matches ambud-agent's default --listen address.
const defaultAgentURL = "http://localhost:8080"

// newRootCmd builds the ambudctl command tree. Each subcommand takes a
// runtimeFactory rather than calling apiclient.New directly, so tests
// can substitute a runtime.Fake instead of needing a live agent — see
// docs/ROADMAP.md's Phase 1 note on keeping runtime-dependent logic
// testable without a live daemon; the same principle now applies one
// layer further out, to the agent's HTTP API.
func newRootCmd() *cobra.Command {
	var agentURL string

	root := &cobra.Command{
		Use:   "ambudctl",
		Short: "ambudctl is the Ambud command-line client",
		Long: `ambudctl is the Ambud command-line client.

As of Phase 2 (docs/ROADMAP.md), ambudctl talks to a local ambud-agent
over HTTP rather than driving containerd directly — that was Phase 1's
scaffolding, now superseded now that an agent exists. There is still no
control plane (Phase 3): --agent must point at one specific machine's
agent, and "ambudctl node list"-style cluster commands don't exist yet.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&agentURL, "agent", defaultAgentURL,
		"ambud-agent URL")

	factory := func() (runtime.Runtime, error) {
		return apiclient.New(agentURL), nil
	}

	root.AddCommand(newRunCmd(factory))
	root.AddCommand(newPSCmd(factory))
	root.AddCommand(newStopCmd(factory))
	root.AddCommand(newVersionCmd())

	return root
}

// runtimeFactory produces a Runtime for a single command invocation.
// The production factory (above) talks to a real agent over HTTP;
// tests inject one that returns a runtime.Fake.
type runtimeFactory func() (runtime.Runtime, error)
