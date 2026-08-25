// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStopCmd(newRuntime runtimeFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "stop NAME",
		Short: "Stop and remove a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			rt, err := newRuntime()
			if err != nil {
				return fmt.Errorf("connect to runtime: %w", err)
			}
			defer func() { _ = rt.Close() }()

			if stopErr := rt.Stop(cmd.Context(), name); stopErr != nil {
				return fmt.Errorf("stop %s: %w", name, stopErr)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "stopped %s\n", name)
			return err
		},
	}
}
