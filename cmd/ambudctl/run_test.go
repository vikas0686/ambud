// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/vikas0686/ambud/internal/runtime"
)

func TestDefaultContainerName(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{name: "simple", image: "nginx", want: "nginx"},
		{name: "with tag", image: "nginx:alpine", want: "nginx"},
		{name: "with library namespace", image: "docker.io/library/nginx:alpine", want: "nginx"},
		{name: "with digest", image: "nginx@sha256:abcd1234", want: "nginx"},
		{name: "with registry port and tag", image: "myregistry:5000/team/app:v1.2.3", want: "app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultContainerName(tt.image); got != tt.want {
				t.Errorf("defaultContainerName(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestRunCmd_DerivesNameAndStartsContainer(t *testing.T) {
	fake := runtime.NewFake()
	cmd := newRunCmd(fakeFactory(fake))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"docker.io/library/nginx:alpine"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got, want := out.String(), "started nginx (docker.io/library/nginx:alpine)\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}

	statuses, err := fake.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "nginx" {
		t.Errorf("List() = %+v, want one container named nginx", statuses)
	}
}

func TestRunCmd_ExplicitNameOverridesDerived(t *testing.T) {
	fake := runtime.NewFake()
	cmd := newRunCmd(fakeFactory(fake))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--name", "web-1", "nginx:alpine"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	statuses, _ := fake.List(context.Background())
	if len(statuses) != 1 || statuses[0].Name != "web-1" {
		t.Errorf("List() = %+v, want one container named web-1", statuses)
	}
}

func TestRunCmd_AlreadyExistsSurfacesAsError(t *testing.T) {
	fake := runtime.NewFake()
	if err := fake.Run(context.Background(), "nginx", "nginx:alpine", nil); err != nil {
		t.Fatalf("seeding fake failed: %v", err)
	}

	cmd := newRunCmd(fakeFactory(fake))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"nginx:alpine"})

	err := cmd.Execute()
	if !errors.Is(err, runtime.ErrAlreadyExists) {
		t.Errorf("Execute() error = %v, want wrapping runtime.ErrAlreadyExists", err)
	}
}

// fakeFactory adapts an already-constructed runtime.Runtime (typically
// a runtime.Fake) into a runtimeFactory for tests, so command tests
// never need a live containerd socket.
func fakeFactory(rt runtime.Runtime) runtimeFactory {
	return func() (runtime.Runtime, error) {
		return rt, nil
	}
}
