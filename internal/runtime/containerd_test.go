// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"testing"

	containerd "github.com/containerd/containerd/v2/client"
)

func TestMapProcessStatus(t *testing.T) {
	tests := []struct {
		name string
		in   containerd.ProcessStatus
		want State
	}{
		{name: "running", in: containerd.Running, want: StateRunning},
		{name: "stopped", in: containerd.Stopped, want: StateStopped},
		{name: "created maps to unknown", in: containerd.Created, want: StateUnknown},
		{name: "paused maps to unknown", in: containerd.Paused, want: StateUnknown},
		{name: "empty maps to unknown", in: containerd.ProcessStatus(""), want: StateUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapProcessStatus(tt.in); got != tt.want {
				t.Errorf("mapProcessStatus(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
