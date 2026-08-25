// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
)

// controlPlaneAPI is exactly what the reconcile loop (reconciler.go)
// needs from a control plane. It's a narrow interface — not
// internal/apiclient.Client, and not internal/runtime.Runtime — so
// reconciler tests can substitute a fake instead of a live control
// plane HTTP server, same pattern as internal/runtime.Fake.
type controlPlaneAPI interface {
	Register(ctx context.Context, joinToken, name string) (nodeID, credential string, err error)
	Heartbeat(ctx context.Context, nodeID, credential string, req apitypes.HeartbeatRequest) (apitypes.HeartbeatResponse, error)
}

// ControlPlaneClient talks to a real ambud-controlplane over HTTP. It
// satisfies the unexported controlPlaneAPI interface that Reconciler
// depends on.
type ControlPlaneClient struct {
	baseURL string
	http    *http.Client
}

var _ controlPlaneAPI = (*ControlPlaneClient)(nil)

// NewControlPlaneClient returns a client for the ambud-controlplane at
// baseURL.
func NewControlPlaneClient(baseURL string) *ControlPlaneClient {
	return &ControlPlaneClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Register implements controlPlaneAPI.
func (c *ControlPlaneClient) Register(ctx context.Context, joinToken, name string) (string, string, error) {
	var resp apitypes.RegisterNodeResponse
	err := c.doJSON(ctx, http.MethodPost, "/v1/nodes/register", "",
		apitypes.RegisterNodeRequest{JoinToken: joinToken, Name: name}, &resp)
	if err != nil {
		return "", "", err
	}
	return resp.NodeID, resp.Credential, nil
}

// Heartbeat implements controlPlaneAPI.
func (c *ControlPlaneClient) Heartbeat(ctx context.Context, nodeID, credential string, req apitypes.HeartbeatRequest) (apitypes.HeartbeatResponse, error) {
	var resp apitypes.HeartbeatResponse
	err := c.doJSON(ctx, http.MethodPost, "/v1/nodes/"+nodeID+"/heartbeat", credential, req, &resp)
	return resp, err
}

func (c *ControlPlaneClient) doJSON(ctx context.Context, method, path, bearerToken string, body, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
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
		return fmt.Errorf("%s %s: %s", method, path, msg)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
