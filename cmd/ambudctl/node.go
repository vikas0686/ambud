// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func newNodeCmd(cp controlPlaneFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage cluster nodes",
	}
	cmd.AddCommand(newNodeListCmd(cp))
	cmd.AddCommand(newNodeGenerateJoinTokenCmd(cp))
	return cmd
}

func newNodeListCmd(cp controlPlaneFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered nodes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := cp()
			if err != nil {
				return fmt.Errorf("connect to control plane: %w", err)
			}

			nodes, err := client.ListNodes(cmd.Context())
			if err != nil {
				return fmt.Errorf("list nodes: %w", err)
			}
			return printNodes(cmd.OutOrStdout(), nodes)
		},
	}
}

func newNodeGenerateJoinTokenCmd(cp controlPlaneFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "generate-join-token",
		Short: "Generate a one-time token for a new node to register with",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := cp()
			if err != nil {
				return fmt.Errorf("connect to control plane: %w", err)
			}

			token, err := client.CreateJoinToken(cmd.Context())
			if err != nil {
				return fmt.Errorf("generate join token: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), token)
			return err
		},
	}
}

func printNodes(w io.Writer, nodes []apitypes.NodeStatus) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, "NAME\tLAST HEARTBEAT\tCPU CORES\tCPU %\tMEM USED\tMEM TOTAL"); err != nil {
		return err
	}
	for _, n := range nodes {
		lastHeartbeat := "never"
		if n.LastHeartbeatAt != nil {
			lastHeartbeat = time.Since(*n.LastHeartbeatAt).Round(time.Second).String() + " ago"
		}
		_, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%.1f\t%d\t%d\n",
			n.Name, lastHeartbeat, n.Resources.CPUCores, n.Resources.CPUUsedPercent,
			n.Resources.MemUsedBytes, n.Resources.MemTotalBytes)
		if err != nil {
			return err
		}
	}

	return tw.Flush()
}
