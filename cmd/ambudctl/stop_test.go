// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/vikas0686/ambud/internal/runtime"
)

func TestStopCmd_StopsRunningContainer(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}

	cmd := newStopCmd(fakeFactory(fake))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"web"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got, want := out.String(), "stopped web\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}

	// Stop removes the container entirely (matching Containerd.Stop),
	// so it no longer appears in List.
	statuses, _ := fake.List(context.Background())
	if len(statuses) != 0 {
		t.Errorf("List() after stop = %+v, want empty", statuses)
	}
}

func TestStopCmd_UnknownContainerSurfacesAsError(t *testing.T) {
	cmd := newStopCmd(fakeFactory(runtime.NewFake()))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"does-not-exist"})

	err := cmd.Execute()
	if !errors.Is(err, runtime.ErrNotFound) {
		t.Errorf("Execute() error = %v, want wrapping runtime.ErrNotFound", err)
	}
}
