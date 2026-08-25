// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/apiclient"
	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/cpclient"
	"github.com/vikas0686/ambud/internal/runtime"
)

// defaultAgentURL matches ambud-agent's default --listen address.
const defaultAgentURL = "http://localhost:8080"

// defaultControlPlaneURL matches ambud-controlplane's default --listen
// address.
const defaultControlPlaneURL = "http://localhost:8081"

// newRootCmd builds the ambudctl command tree. Subcommands take a
// runtimeFactory or controlPlaneFactory rather than calling
// apiclient.New/cpclient.New directly, so tests can substitute a fake
// instead of needing a live agent or control plane — see
// docs/ROADMAP.md's Phase 1 note on keeping runtime-dependent logic
// testable without a live daemon; the same principle applies to every
// network dependency ambudctl has gained since.
func newRootCmd() *cobra.Command {
	var agentURL, controlPlaneURL string

	root := &cobra.Command{
		Use:   "ambudctl",
		Short: "ambudctl is the Ambud command-line client",
		Long: `ambudctl is the Ambud command-line client.

"run"/"ps"/"stop" talk directly to one ambud-agent (--agent) — useful
for low-level debugging of a single machine, same as Phase 2.

"node"/"deploy"/"workloads" talk to the control plane (--controlplane,
Phase 3 onward) and operate on the whole cluster: registering nodes,
deploying workloads, and seeing what's running where.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&agentURL, "agent", defaultAgentURL,
		"ambud-agent URL, for run/ps/stop")
	root.PersistentFlags().StringVar(&controlPlaneURL, "controlplane", defaultControlPlaneURL,
		"ambud-controlplane URL, for node/deploy/workloads")

	runtimeF := func() (runtime.Runtime, error) {
		return apiclient.New(agentURL), nil
	}
	controlPlaneF := func() (controlPlaneAPI, error) {
		return cpclient.New(controlPlaneURL), nil
	}

	root.AddCommand(newRunCmd(runtimeF))
	root.AddCommand(newPSCmd(runtimeF))
	root.AddCommand(newStopCmd(runtimeF))
	root.AddCommand(newVersionCmd())

	root.AddCommand(newNodeCmd(controlPlaneF))
	root.AddCommand(newDeployCmd(controlPlaneF))
	root.AddCommand(newWorkloadsCmd(controlPlaneF))

	return root
}

// runtimeFactory produces a Runtime for a single command invocation.
// The production factory (above) talks to a real agent over HTTP;
// tests inject one that returns a runtime.Fake.
type runtimeFactory func() (runtime.Runtime, error)

// controlPlaneAPI is exactly what ambudctl's node/deploy/workloads
// commands need from a control plane — narrow on purpose, matching
// runtimeFactory's rationale, so tests can substitute a fake instead
// of *cpclient.Client (a concrete type, not swappable on its own).
type controlPlaneAPI interface {
	CreateJoinToken(ctx context.Context) (string, error)
	ListNodes(ctx context.Context) ([]apitypes.NodeStatus, error)
	CreateWorkload(ctx context.Context, req apitypes.CreateWorkloadRequest) (apitypes.WorkloadStatus, error)
	ListWorkloads(ctx context.Context) ([]apitypes.WorkloadStatus, error)
}

// controlPlaneFactory produces a controlPlaneAPI for a single command
// invocation. The production factory (above) talks to a real control
// plane over HTTP; tests inject one that returns a fake.
type controlPlaneFactory func() (controlPlaneAPI, error)
