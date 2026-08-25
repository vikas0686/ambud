// SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func TestNodeState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	const timeout = 15 * time.Second

	tests := []struct {
		name            string
		lastHeartbeatAt *time.Time
		want            apitypes.NodeState
	}{
		{
			name:            "never heartbeated",
			lastHeartbeatAt: nil,
			want:            apitypes.NodeOffline,
		},
		{
			name:            "just heartbeated",
			lastHeartbeatAt: ptr(now),
			want:            apitypes.NodeOnline,
		},
		{
			name:            "heartbeated within timeout",
			lastHeartbeatAt: ptr(now.Add(-10 * time.Second)),
			want:            apitypes.NodeOnline,
		},
		{
			name:            "heartbeated exactly at timeout boundary",
			lastHeartbeatAt: ptr(now.Add(-timeout)),
			want:            apitypes.NodeOnline,
		},
		{
			name:            "heartbeated just past timeout",
			lastHeartbeatAt: ptr(now.Add(-timeout - time.Second)),
			want:            apitypes.NodeOffline,
		},
		{
			name:            "heartbeated long ago",
			lastHeartbeatAt: ptr(now.Add(-time.Hour)),
			want:            apitypes.NodeOffline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeState(tt.lastHeartbeatAt, timeout, now); got != tt.want {
				t.Errorf("nodeState(%v, %v, %v) = %q, want %q", tt.lastHeartbeatAt, timeout, now, got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
