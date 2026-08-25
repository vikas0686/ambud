// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"
)

func TestDeployCmd_DerivesNameAndDeploys(t *testing.T) {
	cp := &fakeControlPlane{}
	cmd := newDeployCmd(fakeControlPlaneFactory(cp))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"docker.io/library/nginx:alpine"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if cp.lastCreateWorkR.Name != "nginx" || cp.lastCreateWorkR.Image != "docker.io/library/nginx:alpine" {
		t.Errorf("CreateWorkload called with %+v, want name=nginx", cp.lastCreateWorkR)
	}
	if cp.lastCreateWorkR.NodeID != "" {
		t.Errorf("CreateWorkload called with NodeID=%q, want empty when --node is omitted", cp.lastCreateWorkR.NodeID)
	}
}

func TestDeployCmd_ExplicitNameAndNode(t *testing.T) {
	cp := &fakeControlPlane{}
	cmd := newDeployCmd(fakeControlPlaneFactory(cp))

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--name", "web-1", "--node", "node-abc", "nginx:alpine"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if cp.lastCreateWorkR.Name != "web-1" || cp.lastCreateWorkR.NodeID != "node-abc" {
		t.Errorf("CreateWorkload called with %+v, want name=web-1 node=node-abc", cp.lastCreateWorkR)
	}
}

func TestDeployCmd_PropagatesError(t *testing.T) {
	cp := &fakeControlPlane{createWorkErr: errFakeControlPlane}
	cmd := newDeployCmd(fakeControlPlaneFactory(cp))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"nginx:alpine"})

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() error = nil, want the fake's error to propagate")
	}
}
