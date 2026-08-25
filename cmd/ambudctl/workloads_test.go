// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vikas0686/ambud/internal/apitypes"
)

func TestWorkloadsListCmd(t *testing.T) {
	cp := &fakeControlPlane{
		workloads: []apitypes.WorkloadStatus{
			{Name: "web", Image: "nginx:alpine", NodeName: "node-1", State: "running", PID: 4242},
		},
	}
	cmd := newWorkloadsListCmd(fakeControlPlaneFactory(cp))

	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	got := out.String()
	for _, want := range []string{"web", "nginx:alpine", "node-1", "running", "4242"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

func TestWorkloadsListCmd_PropagatesError(t *testing.T) {
	cp := &fakeControlPlane{listWorkErr: errFakeControlPlane}
	cmd := newWorkloadsListCmd(fakeControlPlaneFactory(cp))
	cmd.SetOut(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() error = nil, want the fake's error to propagate")
	}
}
