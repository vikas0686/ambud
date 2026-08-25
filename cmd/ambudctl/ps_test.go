// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/vikas0686/ambud/internal/runtime"
)

func TestPrintStatuses(t *testing.T) {
	var out bytes.Buffer
	statuses := []runtime.ContainerStatus{
		{Name: "web", Image: "nginx:alpine", State: runtime.StateRunning, PID: 4242},
		{Name: "cache", Image: "redis:alpine", State: runtime.StateStopped, PID: 0},
	}

	if err := printStatuses(&out, statuses); err != nil {
		t.Fatalf("printStatuses() error = %v, want nil", err)
	}

	got := out.String()
	for _, want := range []string{"NAME", "IMAGE", "STATE", "PID", "web", "nginx:alpine", "running", "4242", "cache", "stopped"} {
		if !strings.Contains(got, want) {
			t.Errorf("printStatuses() output missing %q; got:\n%s", want, got)
		}
	}
}

func TestPrintStatuses_Empty(t *testing.T) {
	var out bytes.Buffer
	if err := printStatuses(&out, nil); err != nil {
		t.Fatalf("printStatuses(nil) error = %v, want nil", err)
	}
	if got := out.String(); !strings.Contains(got, "NAME") {
		t.Errorf("printStatuses(nil) = %q, want header row even with no containers", got)
	}
}

func TestPSCmd_ListsRunningContainers(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}

	cmd := newPSCmd(fakeFactory(fake))
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := out.String(); !strings.Contains(got, "web") || !strings.Contains(got, "nginx:alpine") {
		t.Errorf("ps output = %q, want it to contain the seeded container", got)
	}
}
