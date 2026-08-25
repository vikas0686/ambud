// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"fmt"
)

// CreateJoinToken generates a new one-time join token and persists its
// hash. It returns the plaintext token — the only time it's ever
// available; RegisterNode later consumes it by its hash alone.
func (s *Store) CreateJoinToken(ctx context.Context) (string, error) {
	token, hash, err := newCredential() // a join token is just a random token, same shape as a node credential
	if err != nil {
		return "", err
	}

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO join_tokens (token_hash) VALUES ($1)`,
		hash,
	); err != nil {
		return "", fmt.Errorf("create join token: %w", err)
	}
	return token, nil
}
