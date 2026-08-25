// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func newDeployCmd(cp controlPlaneFactory) *cobra.Command {
	var name, nodeID string

	cmd := &cobra.Command{
		Use:   "deploy IMAGE",
		Short: "Deploy a workload to the cluster",
		Long: `Deploy a workload to the cluster.

If --node is omitted, the control plane assigns the only registered
node when there's exactly one, and errors otherwise — there's no
scheduler yet (see docs/ROADMAP.md Phase 5).

The workload doesn't start immediately: it's picked up and reconciled
on the assigned node's next heartbeat. Check "ambudctl workloads list"
or "ambudctl node list" shortly after to see it running.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			image := args[0]
			workloadName := name
			if workloadName == "" {
				workloadName = defaultContainerName(image)
			}

			client, err := cp()
			if err != nil {
				return fmt.Errorf("connect to control plane: %w", err)
			}

			w, err := client.CreateWorkload(cmd.Context(), apitypes.CreateWorkloadRequest{
				Name: workloadName, Image: image, NodeID: nodeID,
			})
			if err != nil {
				return fmt.Errorf("deploy %s: %w", workloadName, err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "deployed %s (%s) to node %s\n", w.Name, w.Image, w.NodeID)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "workload name (default: derived from the image name)")
	cmd.Flags().StringVar(&nodeID, "node", "", "target node ID (default: the only registered node, if there's exactly one)")
	return cmd
}
