// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRunCmd(newRuntime runtimeFactory) *cobra.Command {
	var name string
	var portSpecs []string

	cmd := &cobra.Command{
		Use:   "run IMAGE",
		Short: "Pull an image and run it as a new container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			image := args[0]
			// containerName is derived from name (the --name flag)
			// without mutating it, so repeated invocations of this
			// command (e.g. across table-driven tests that reuse a
			// command tree) always derive consistently rather than
			// depending on what a previous invocation left behind.
			containerName := name
			if containerName == "" {
				containerName = defaultContainerName(image)
			}

			ports, err := parsePortSpecs(portSpecs)
			if err != nil {
				return err
			}

			rt, err := newRuntime()
			if err != nil {
				return fmt.Errorf("connect to runtime: %w", err)
			}
			defer func() { _ = rt.Close() }()

			if runErr := rt.Run(cmd.Context(), containerName, image, ports); runErr != nil {
				return fmt.Errorf("run %s: %w", containerName, runErr)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "started %s (%s)\n", containerName, image)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "container name (default: derived from the image name)")
	cmd.Flags().StringArrayVar(&portSpecs, "port", nil,
		"publish a container port to the host, as hostPort:containerPort[/protocol] (repeatable)")
	return cmd
}

// defaultContainerName derives a container name from an image
// reference when the caller doesn't pass --name, e.g.
// "docker.io/library/nginx:alpine" -> "nginx".
func defaultContainerName(image string) string {
	ref := image
	if i := strings.IndexByte(ref, '@'); i != -1 {
		ref = ref[:i] // strip a digest, e.g. "...@sha256:..."
	}
	if i := strings.LastIndexByte(ref, '/'); i != -1 {
		ref = ref[i+1:]
	}
	if i := strings.LastIndexByte(ref, ':'); i != -1 {
		ref = ref[:i] // strip the tag
	}
	return ref
}
