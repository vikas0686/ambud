// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/runtime"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeControlPlane is an in-memory controlPlaneAPI for reconciler
// tests — no HTTP, no real control plane. heartbeatCh fires after
// every Heartbeat call so tests can deterministically wait for one
// heartbeat cycle instead of sleeping.
type fakeControlPlane struct {
	mu sync.Mutex

	nodeID, credential string
	registerErr        error
	registerCalls      int
	registeredToken    string
	registeredName     string

	heartbeatResp apitypes.HeartbeatResponse
	heartbeatErr  error
	heartbeatCh   chan apitypes.HeartbeatRequest
}

func newFakeControlPlane() *fakeControlPlane {
	return &fakeControlPlane{
		nodeID: "node-1", credential: "secret",
		heartbeatCh: make(chan apitypes.HeartbeatRequest, 16),
	}
}

func (f *fakeControlPlane) Register(_ context.Context, joinToken, name string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerCalls++
	f.registeredToken = joinToken
	f.registeredName = name
	if f.registerErr != nil {
		return "", "", f.registerErr
	}
	return f.nodeID, f.credential, nil
}

func (f *fakeControlPlane) Heartbeat(_ context.Context, _, _ string, req apitypes.HeartbeatRequest) (apitypes.HeartbeatResponse, error) {
	f.mu.Lock()
	resp, err := f.heartbeatResp, f.heartbeatErr
	f.mu.Unlock()
	f.heartbeatCh <- req
	return resp, err
}

func (f *fakeControlPlane) waitForHeartbeat(t *testing.T) apitypes.HeartbeatRequest {
	t.Helper()
	select {
	case req := <-f.heartbeatCh:
		return req
	case <-time.After(2 * time.Second):
		t.Fatal("no heartbeat received within 2s")
		return apitypes.HeartbeatRequest{}
	}
}

func TestReconciler_RegistersWhenNoCredentialExists(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	cp := newFakeControlPlane()
	rt := runtime.NewFake()
	collector := NewResourceCollector(time.Hour, "/")

	r := NewReconciler(cp, rt, collector, credPath, "join-tok", "my-node", time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	cp.waitForHeartbeat(t)
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run() = %v, want nil after cancellation", err)
	}

	if cp.registerCalls != 1 {
		t.Errorf("Register called %d times, want 1", cp.registerCalls)
	}
	if cp.registeredToken != "join-tok" || cp.registeredName != "my-node" {
		t.Errorf("Register called with (%q, %q), want (join-tok, my-node)", cp.registeredToken, cp.registeredName)
	}

	cred, found, err := LoadCredential(credPath)
	if err != nil || !found {
		t.Fatalf("LoadCredential() = (%+v, %v, %v), want found", cred, found, err)
	}
	if cred.NodeID != "node-1" || cred.Credential != "secret" {
		t.Errorf("saved credential = %+v, want node-1/secret", cred)
	}
}

func TestReconciler_UsesSavedCredentialWithoutReregistering(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	if err := SaveCredential(credPath, NodeCredential{NodeID: "existing-node", Credential: "existing-cred"}); err != nil {
		t.Fatalf("SaveCredential() error = %v", err)
	}

	cp := newFakeControlPlane()
	rt := runtime.NewFake()
	collector := NewResourceCollector(time.Hour, "/")
	// No join token provided — must not be needed, since a credential
	// already exists on disk.
	r := NewReconciler(cp, rt, collector, credPath, "", "my-node", time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	cp.waitForHeartbeat(t)
	cancel()
	<-done

	if cp.registerCalls != 0 {
		t.Errorf("Register called %d times, want 0 (should reuse saved credential)", cp.registerCalls)
	}
}

func TestReconciler_NoCredentialAndNoJoinTokenFails(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	cp := newFakeControlPlane()
	rt := runtime.NewFake()
	collector := NewResourceCollector(time.Hour, "/")
	r := NewReconciler(cp, rt, collector, credPath, "", "my-node", time.Hour, testLogger())

	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want an error when there's no credential and no join token")
	}
}

func TestReconciler_StartsDesiredWorkloadNotYetRunning(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	cp := newFakeControlPlane()
	cp.heartbeatResp = apitypes.HeartbeatResponse{
		Workloads: []apitypes.WorkloadSpec{{Name: "web", Image: "nginx:alpine"}},
	}
	rt := runtime.NewFake()
	collector := NewResourceCollector(time.Hour, "/")
	r := NewReconciler(cp, rt, collector, credPath, "join-tok", "my-node", time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	cp.waitForHeartbeat(t)
	// The workload is started as a side effect of the heartbeat that
	// already fired — no need to wait for a second cycle.
	cancel()
	<-done

	statuses, err := rt.List(context.Background())
	if err != nil {
		t.Fatalf("rt.List() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "web" {
		t.Errorf("rt.List() = %+v, want the desired workload \"web\" running", statuses)
	}
}

func TestReconciler_RestartsCrashedWorkload(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	cp := newFakeControlPlane()
	cp.heartbeatResp = apitypes.HeartbeatResponse{
		Workloads: []apitypes.WorkloadSpec{{Name: "web", Image: "nginx:alpine"}},
	}
	rt := runtime.NewFake()
	if err := rt.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seed rt.Run() error = %v", err)
	}
	// Simulate the container's process dying on its own — the record
	// stays around as Stopped, distinct from having been Stop()'d (which
	// would remove it). Before the fix, reconcile only checked "does a
	// record exist by this name," so a crashed-but-present container
	// was wrongly treated as already satisfied and never restarted.
	rt.SimulateCrash("web")

	collector := NewResourceCollector(time.Hour, "/")
	r := NewReconciler(cp, rt, collector, credPath, "join-tok", "my-node", time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	cp.waitForHeartbeat(t)
	cancel()
	<-done

	statuses, err := rt.List(context.Background())
	if err != nil {
		t.Fatalf("rt.List() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "web" || statuses[0].State != runtime.StateRunning {
		t.Errorf("rt.List() = %+v, want web running again after reconcile", statuses)
	}
}

func TestReconciler_DoesNotRestartAlreadyRunningWorkload(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	cp := newFakeControlPlane()
	cp.heartbeatResp = apitypes.HeartbeatResponse{
		Workloads: []apitypes.WorkloadSpec{{Name: "web", Image: "nginx:alpine"}},
	}
	rt := runtime.NewFake()
	if err := rt.Run(context.Background(), "web", "nginx:alpine"); err != nil {
		t.Fatalf("seed rt.Run() error = %v", err)
	}
	collector := NewResourceCollector(time.Hour, "/")
	r := NewReconciler(cp, rt, collector, credPath, "join-tok", "my-node", time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	req := cp.waitForHeartbeat(t)
	cancel()
	<-done

	// The already-running container must have been reported in the
	// heartbeat itself (that's how the control plane would learn its
	// real state), and reconcile must not have tried to recreate it —
	// runtime.Fake.Run on an existing name would have errored, and that
	// error would show up as a log line, not a test failure, so assert
	// the positive instead: exactly one container, still running.
	if len(req.Containers) != 1 || req.Containers[0].Name != "web" {
		t.Errorf("heartbeat reported containers = %+v, want just web", req.Containers)
	}
	statuses, _ := rt.List(context.Background())
	if len(statuses) != 1 || statuses[0].State != runtime.StateRunning {
		t.Errorf("rt.List() = %+v, want web still running (untouched)", statuses)
	}
}

func TestReconciler_HeartbeatFailureDoesNotStopTheLoop(t *testing.T) {
	credPath := filepath.Join(t.TempDir(), "credentials.json")
	cp := newFakeControlPlane()
	cp.heartbeatErr = errors.New("control plane unreachable")
	rt := runtime.NewFake()
	collector := NewResourceCollector(time.Hour, "/")
	r := NewReconciler(cp, rt, collector, credPath, "join-tok", "my-node", 10*time.Millisecond, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	// A failed heartbeat must not stop the loop — see
	// docs/ARCHITECTURE.md's "fail static" principle. Confirm at least
	// two heartbeats happen despite every one failing.
	cp.waitForHeartbeat(t)
	cp.waitForHeartbeat(t)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil (heartbeat failures are logged, not fatal)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return within 2s of cancellation")
	}
}
