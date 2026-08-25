// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultCredentialsPath is where the agent persists its node
// credential after registering with a control plane, so it doesn't
// need to consume another (single-use) join token on every restart.
const DefaultCredentialsPath = "/var/lib/ambud/agent/credentials.json" //nolint:gosec // a file path, not a credential value

// NodeCredential is what the agent persists locally after a successful
// registration with a control plane.
type NodeCredential struct {
	NodeID     string `json:"node_id"`
	Credential string `json:"credential"`
}

// LoadCredential reads a previously saved credential from path. A
// missing file is reported as (zero value, false, nil) — the ordinary
// "never registered" case, not an error.
func LoadCredential(path string) (NodeCredential, bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is an operator-controlled config value (a CLI flag), not untrusted input
	if errors.Is(err, os.ErrNotExist) {
		return NodeCredential{}, false, nil
	}
	if err != nil {
		return NodeCredential{}, false, fmt.Errorf("read credential file %s: %w", path, err)
	}

	var cred NodeCredential
	if err := json.Unmarshal(data, &cred); err != nil {
		return NodeCredential{}, false, fmt.Errorf("parse credential file %s: %w", path, err)
	}
	return cred, true, nil
}

// SaveCredential persists cred to path, creating parent directories as
// needed. File permissions are locked down to owner-only (0600) since
// Credential is a live bearer token — anyone who reads this file can
// heartbeat as this node.
func SaveCredential(path string, cred NodeCredential) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create credential directory for %s: %w", path, err)
	}

	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credential: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write credential file %s: %w", path, err)
	}
	return nil
}
