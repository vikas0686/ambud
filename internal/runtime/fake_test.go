// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestFake_RunListStop(t *testing.T) {
	ctx := context.Background()
	f := NewFake()

	if err := f.Run(ctx, "web", "nginx:alpine"); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	statuses, err := f.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("List() returned %d containers, want 1", len(statuses))
	}
	if got := statuses[0]; got.Name != "web" || got.Image != "nginx:alpine" || got.State != StateRunning {
		t.Errorf("List()[0] = %+v, want {web nginx:alpine running ...}", got)
	}

	if stopErr := f.Stop(ctx, "web"); stopErr != nil {
		t.Fatalf("Stop() = %v, want nil", stopErr)
	}

	statuses, err = f.List(ctx)
	if err != nil {
		t.Fatalf("List() after Stop error = %v, want nil", err)
	}
	if len(statuses) != 1 || statuses[0].State != StateStopped {
		t.Errorf("List() after Stop = %+v, want state %q", statuses, StateStopped)
	}
}

func TestFake_RunDuplicateNameFails(t *testing.T) {
	ctx := context.Background()
	f := NewFake()

	if err := f.Run(ctx, "web", "nginx:alpine"); err != nil {
		t.Fatalf("first Run() = %v, want nil", err)
	}

	err := f.Run(ctx, "web", "nginx:alpine")
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("second Run() error = %v, want wrapping ErrAlreadyExists", err)
	}
}

func TestFake_StopUnknownContainerFails(t *testing.T) {
	err := NewFake().Stop(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Stop() error = %v, want wrapping ErrNotFound", err)
	}
}

func TestFake_RunPropagatesPullErr(t *testing.T) {
	f := NewFake()
	wantErr := errors.New("registry unreachable")
	f.PullErr = wantErr

	err := f.Run(context.Background(), "web", "nginx:alpine")
	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want wrapping %v", err, wantErr)
	}

	statuses, _ := f.List(context.Background())
	if len(statuses) != 0 {
		t.Errorf("List() = %v, want empty after failed Run", statuses)
	}
}
