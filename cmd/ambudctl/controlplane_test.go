// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/vikas0686/ambud/internal/apitypes"
)

// fakeControlPlane is an in-memory controlPlaneAPI for command tests —
// no HTTP, no real control plane, same pattern as run_test.go's use of
// runtime.Fake via fakeFactory.
type fakeControlPlane struct {
	nodes           []apitypes.NodeStatus
	workloads       []apitypes.WorkloadStatus
	joinToken       string
	createTokenErr  error
	listNodesErr    error
	createWorkErr   error
	listWorkErr     error
	lastCreateWorkR apitypes.CreateWorkloadRequest
}

func (f *fakeControlPlane) CreateJoinToken(context.Context) (string, error) {
	if f.createTokenErr != nil {
		return "", f.createTokenErr
	}
	return f.joinToken, nil
}

func (f *fakeControlPlane) ListNodes(context.Context) ([]apitypes.NodeStatus, error) {
	if f.listNodesErr != nil {
		return nil, f.listNodesErr
	}
	return f.nodes, nil
}

func (f *fakeControlPlane) CreateWorkload(_ context.Context, req apitypes.CreateWorkloadRequest) (apitypes.WorkloadStatus, error) {
	f.lastCreateWorkR = req
	if f.createWorkErr != nil {
		return apitypes.WorkloadStatus{}, f.createWorkErr
	}
	return apitypes.WorkloadStatus{Name: req.Name, Image: req.Image, NodeID: req.NodeID}, nil
}

func (f *fakeControlPlane) ListWorkloads(context.Context) ([]apitypes.WorkloadStatus, error) {
	if f.listWorkErr != nil {
		return nil, f.listWorkErr
	}
	return f.workloads, nil
}

func fakeControlPlaneFactory(cp *fakeControlPlane) controlPlaneFactory {
	return func() (controlPlaneAPI, error) {
		return cp, nil
	}
}

var errFakeControlPlane = fmt.Errorf("fake control plane error")
