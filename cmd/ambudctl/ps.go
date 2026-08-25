// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/vikas0686/ambud/internal/runtime"
)

func newPSCmd(newRuntime runtimeFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List containers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := newRuntime()
			if err != nil {
				return fmt.Errorf("connect to runtime: %w", err)
			}
			defer func() { _ = rt.Close() }()

			statuses, err := rt.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("list containers: %w", err)
			}

			return printStatuses(cmd.OutOrStdout(), statuses)
		},
	}
}

// printStatuses renders statuses as an aligned, tab-separated table.
func printStatuses(w io.Writer, statuses []runtime.ContainerStatus) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, "NAME\tIMAGE\tSTATE\tPID\tPORTS"); err != nil {
		return err
	}
	for _, st := range statuses {
		_, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", st.Name, st.Image, st.State, st.PID, formatRuntimePorts(st.Ports))
		if err != nil {
			return err
		}
	}

	return tw.Flush()
}
