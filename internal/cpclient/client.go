// SPDX-License-Identifier: Apache-2.0

// Package cpclient is an HTTP client for the ambud-controlplane REST
// API's cluster-management surface (nodes, workloads, join tokens) —
// what ambudctl's `node`/`deploy`/`workloads` commands use, and what
// the Web UI will call once it exists (Phase 3's node list page and
// deploy form — see docs/ROADMAP.md). It's a separate package from
// internal/apiclient, which talks to one agent's API instead — the two
// have no overlapping endpoints and no reason to share a client type.
package cpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
)

// Client talks to one ambud-controlplane's HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client for the control plane at baseURL (e.g.
// "http://localhost:8081").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateJoinToken requests a new one-time node join token.
func (c *Client) CreateJoinToken(ctx context.Context) (string, error) {
	var resp apitypes.CreateJoinTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/join-tokens", nil, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// ListNodes returns every registered node.
func (c *Client) ListNodes(ctx context.Context) ([]apitypes.NodeStatus, error) {
	var resp apitypes.ListNodesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/nodes", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Nodes, nil
}

// CreateWorkload deploys a new workload. req.NodeID may be empty to
// let the control plane auto-assign the sole registered node — see
// apitypes.CreateWorkloadRequest.
func (c *Client) CreateWorkload(ctx context.Context, req apitypes.CreateWorkloadRequest) (apitypes.WorkloadStatus, error) {
	var resp apitypes.WorkloadStatus
	if err := c.doJSON(ctx, http.MethodPost, "/v1/workloads", req, &resp); err != nil {
		return apitypes.WorkloadStatus{}, err
	}
	return resp, nil
}

// ListWorkloads returns every workload.
func (c *Client) ListWorkloads(ctx context.Context) ([]apitypes.WorkloadStatus, error) {
	var resp apitypes.ListWorkloadsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/workloads", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Workloads, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		var apiErr apitypes.ErrorResponse
		msg := resp.Status
		if decodeErr := json.NewDecoder(resp.Body).Decode(&apiErr); decodeErr == nil && apiErr.Error != "" {
			msg = apiErr.Error
		}
		return errors.New(msg)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
