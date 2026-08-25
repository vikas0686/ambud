// SPDX-License-Identifier: Apache-2.0

// Package store is the control plane's PostgreSQL access layer — the
// only package in the control plane that talks SQL. Handlers in
// internal/controlplane/api call typed methods here (CreateNode,
// ListNodes, ...) rather than embedding queries themselves, so the SQL
// surface stays auditable in one place — see docs/PROJECT_STRUCTURE.md.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	// Registers the "pgx5://" scheme with migrate's driver registry,
	// used by migrateUp below. Deliberately the pgx-backed driver, not
	// database/postgres (which pulls in lib/pq) — one Postgres driver
	// in the dependency tree, not two.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a lookup by ID, name, or credential
// finds nothing.
var ErrNotFound = errors.New("store: not found")

// ErrAlreadyExists is returned when creating a row would violate a
// uniqueness constraint (a node or workload name already taken, a join
// token already used).
var ErrAlreadyExists = errors.New("store: already exists")

// Store is the control plane's PostgreSQL-backed store. The zero value
// is not usable — construct one with Open.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to Postgres at dsn and applies any pending migrations
// before returning — a fresh database is left schema-ready, an
// up-to-date one is a fast no-op. Migrations are embedded in the
// binary (see migrationsFS), so there's no separate migrations
// directory to ship or point at in production.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if err := migrateUp(dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{pool: pool}, nil
}

func migrateUp(dsn string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, pgx5Scheme(dsn))
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// pgx5Scheme rewrites a standard "postgres://" or "postgresql://" DSN
// (what Open/pgxpool expects) into the "pgx5://" scheme
// golang-migrate's pgx/v5 database driver registers itself under.
// Callers pass one DSN in; this is purely a migrate-library quirk that
// shouldn't leak into Open's own signature.
func pgx5Scheme(dsn string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(dsn, prefix); ok {
			return "pgx5://" + rest
		}
	}
	return dsn
}

// Close releases the underlying connection pool.
func (s *Store) Close() {
	s.pool.Close()
}
