// SPDX-License-Identifier: Apache-2.0

// Command ambudctl is the Ambud command-line client.
//
// At this stage of the project (Phase 1 of docs/ROADMAP.md) it talks
// directly to a local containerd socket — there is no agent or control
// plane yet, no HTTP calls at all. Those arrive in Phase 2 and Phase 3.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Nothing more useful to do if writing the error itself fails;
		// the non-zero exit code below is the real signal.
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
