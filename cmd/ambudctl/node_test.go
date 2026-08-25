// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func TestNodeListCmd(t *testing.T) {
	heartbeat := time.Now().Add(-5 * time.Second)
	cp := &fakeControlPlane{
		nodes: []apitypes.NodeStatus{
			{
				Name:            "node-1",
				Status:          apitypes.NodeOnline,
				LastHeartbeatAt: &heartbeat,
				Resources:       apitypes.Resources{CPUCores: 4, CPUUsedPercent: 12.5, MemUsedBytes: 100, MemTotalBytes: 1000},
			},
		},
	}
	cmd := newNodeListCmd(fakeControlPlaneFactory(cp))

	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	got := out.String()
	for _, want := range []string{"node-1", "online", "ago", "4", "12.5"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

func TestNodeListCmd_NeverHeartbeat(t *testing.T) {
	cp := &fakeControlPlane{
		nodes: []apitypes.NodeStatus{{Name: "node-1", Status: apitypes.NodeOffline}},
	}
	cmd := newNodeListCmd(fakeControlPlaneFactory(cp))

	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "never") {
		t.Errorf("output = %q, want it to show \"never\" for a node with no heartbeat yet", got)
	}
	if !strings.Contains(got, "offline") {
		t.Errorf("output = %q, want it to show \"offline\" for a node that has never heartbeated", got)
	}
}

func TestNodeListCmd_PropagatesError(t *testing.T) {
	cp := &fakeControlPlane{listNodesErr: errFakeControlPlane}
	cmd := newNodeListCmd(fakeControlPlaneFactory(cp))
	cmd.SetOut(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() error = nil, want the fake's error to propagate")
	}
}

func TestNodeGenerateJoinTokenCmd(t *testing.T) {
	cp := &fakeControlPlane{joinToken: "tok-abc123"}
	cmd := newNodeGenerateJoinTokenCmd(fakeControlPlaneFactory(cp))

	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got, want := out.String(), "tok-abc123\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
