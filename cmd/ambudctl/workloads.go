// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
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

	if _, err := fmt.Fprintln(tw, "NAME\tIMAGE\tNODE\tSTATE\tPID"); err != nil {
		return err
	}
	for _, wl := range workloads {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n", wl.Name, wl.Image, wl.NodeName, wl.State, wl.PID); err != nil {
			return err
		}
	}

	return tw.Flush()
}
