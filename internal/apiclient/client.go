// SPDX-License-Identifier: Apache-2.0

// Package apiclient is an HTTP client for the ambud-agent REST API
// (internal/agent). It implements internal/runtime.Runtime by
// translating each call into an HTTP request, so ambudctl's command
// code (cmd/ambudctl/run.go, ps.go, stop.go) works unchanged whether
// it's driving containerd directly (Phase 1) or an agent over the
// network (Phase 2 onward) — see docs/ARCHITECTURE.md's "every
// component replaceable behind its interface" principle.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/runtime"
)

// defaultTimeout bounds a single request. It's generous because Run
// pulls an image server-side before returning, which can take a while
// on a slow link or a large image.
const defaultTimeout = 2 * time.Minute

// Client talks to one ambud-agent's HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

var _ runtime.Runtime = (*Client)(nil)

// New returns a Client for the agent at baseURL (e.g.
// "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// Pull implements runtime.Runtime.
func (c *Client) Pull(ctx context.Context, image string) error {
	return c.doJSON(ctx, http.MethodPost, "/v1/images/pull", apitypes.PullImageRequest{Image: image}, nil)
}

// Run implements runtime.Runtime.
func (c *Client) Run(ctx context.Context, name, image string, ports []runtime.PortMapping) error {
	req := apitypes.CreateContainerRequest{Name: name, Image: image, Ports: toAPIPorts(ports)}
	return c.doJSON(ctx, http.MethodPost, "/v1/containers", req, nil)
}

// Stop implements runtime.Runtime.
func (c *Client) Stop(ctx context.Context, name string) error {
	path := "/v1/containers/" + url.PathEscape(name) + "/stop"
	return c.doJSON(ctx, http.MethodPost, path, nil, nil)
}

// List implements runtime.Runtime.
func (c *Client) List(ctx context.Context) ([]runtime.ContainerStatus, error) {
	var resp apitypes.ListContainersResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/containers", nil, &resp); err != nil {
		return nil, err
	}

	statuses := make([]runtime.ContainerStatus, 0, len(resp.Containers))
	for _, s := range resp.Containers {
		statuses = append(statuses, runtime.ContainerStatus{
			Name:  s.Name,
			Image: s.Image,
			State: runtime.State(s.State),
			PID:   s.PID,
			Ports: fromAPIPorts(s.Ports),
		})
	}
	return statuses, nil
}

// toAPIPorts and fromAPIPorts convert between internal/runtime's
// PortMapping and apitypes' identical-shaped wire type — kept as two
// separate types (rather than one shared across packages) so the wire
// contract can't accidentally change just because the runtime
// package's internals do; see apitypes' package doc.
func toAPIPorts(ports []runtime.PortMapping) []apitypes.PortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]apitypes.PortMapping, len(ports))
	for i, p := range ports {
		out[i] = apitypes.PortMapping{ContainerPort: p.ContainerPort, HostPort: p.HostPort, Protocol: p.Protocol}
	}
	return out
}

func fromAPIPorts(ports []apitypes.PortMapping) []runtime.PortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]runtime.PortMapping, len(ports))
	for i, p := range ports {
		out[i] = runtime.PortMapping{ContainerPort: p.ContainerPort, HostPort: p.HostPort, Protocol: p.Protocol}
	}
	return out
}

// Close implements runtime.Runtime. It's a no-op: Client holds no
// connection to release, only an *http.Client.
func (c *Client) Close() error {
	return nil
}

// doJSON sends body (if non-nil) as a JSON request and, on a 2xx
// response, decodes the body into out (if non-nil). Non-2xx responses
// are translated into an error — wrapping runtime.ErrAlreadyExists or
// runtime.ErrNotFound for the status codes internal/agent uses for
// those conditions, so callers written against internal/runtime's
// error contract (e.g. cmd/ambudctl) work identically whether talking
// to containerd directly or to an agent over HTTP.
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
		return responseError(resp)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func responseError(resp *http.Response) error {
	msg := resp.Status
	var apiErr apitypes.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
		msg = apiErr.Error
	}

	switch resp.StatusCode {
	case http.StatusConflict:
		return fmt.Errorf("%s: %w", msg, runtime.ErrAlreadyExists)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", msg, runtime.ErrNotFound)
	default:
		return errors.New(msg)
	}
}
