// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func newWorkloadsCmd(cp controlPlaneFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workloads",
		Short: "Manage cluster workloads",
	}
	cmd.AddCommand(newWorkloadsListCmd(cp))
	return cmd
}

func newWorkloadsListCmd(cp controlPlaneFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workloads",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := cp()
			if err != nil {
				return fmt.Errorf("connect to control plane: %w", err)
			}

			workloads, err := client.ListWorkloads(cmd.Context())
			if err != nil {
				return fmt.Errorf("list workloads: %w", err)
			}
			return printWorkloads(cmd.OutOrStdout(), workloads)
		},
	}
}

func printWorkloads(w io.Writer, workloads []apitypes.WorkloadStatus) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, "NAME\tIMAGE\tNODE\tSTATE\tPID\tADDRESS"); err != nil {
		return err
	}
	for _, wl := range workloads {
		_, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n",
			wl.Name, wl.Image, wl.NodeName, wl.State, wl.PID, formatReachableAddress(wl))
		if err != nil {
			return err
		}
	}

	return tw.Flush()
}

// formatReachableAddress renders where a workload's published ports
// are reachable from outside the cluster — NodeAddress:HostPort per
// mapping, matching what an operator would actually curl/browse to
// (see docs/ROADMAP.md's Phase 6). "-" if the workload published no
// ports, or if the node has no known address yet.
func formatReachableAddress(wl apitypes.WorkloadStatus) string {
	if len(wl.Ports) == 0 || wl.NodeAddress == "" {
		return "-"
	}
	parts := make([]string, len(wl.Ports))
	for i, p := range wl.Ports {
		parts[i] = fmt.Sprintf("%s:%d", wl.NodeAddress, p.HostPort)
	}
	return strings.Join(parts, ",")
}
