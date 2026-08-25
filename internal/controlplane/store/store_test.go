// SPDX-License-Identifier: Apache-2.0

// These tests exercise the real store against a real PostgreSQL
// instance — no fake, no mock — because the whole point of this
// package is the SQL and the constraints it relies on (unique names,
// foreign keys), which a fake can't validate. See
// docs/GO_LEARNING_PATH.md and CI's "go" job, which runs a postgres
// service container specifically so these run on every push, not just
// locally when a contributor happens to have Postgres up.
//
//	Locally: docker run -d -e POSTGRES_USER=ambud -e POSTGRES_PASSWORD=devpassword \
//	  -e POSTGRES_DB=ambud_test -p 15432:5432 postgres:16-alpine
//
// (or `docker compose -f deploy/compose/postgres.yaml up -d` — see docs/DEVELOPMENT.md).
// Override the DSN with AMBUD_TEST_DATABASE_URL if yours differs.
// Tests skip (not fail) if no database is reachable, so `go test ./...`
// still works for a contributor who hasn't set one up yet.
package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// randomUUID returns a UUID guaranteed not to match any row in the
// test database — useful for "unknown ID" test cases.
func randomUUID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}

const defaultTestDSN = "postgres://ambud:devpassword@localhost:15432/ambud_test?sslmode=disable"

func testDSN() string {
	if dsn := os.Getenv("AMBUD_TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return defaultTestDSN
}

// newTestStore opens a real Store, migrated, with all tables truncated
// so each test starts from a clean slate. It skips the test (doesn't
// fail it) if Postgres isn't reachable.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	st, err := Open(ctx, testDSN())
	if err != nil {
		t.Skipf("skipping: no test Postgres reachable at %s: %v", testDSN(), err)
	}
	t.Cleanup(st.Close)

	if _, err := st.pool.Exec(ctx, `TRUNCATE workloads, join_tokens, nodes CASCADE`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	return st
}

func TestOpen_RunsMigrationsIdempotently(t *testing.T) {
	st := newTestStore(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Opening a second time against the same (already-migrated)
	// database must be a no-op, not an error.
	st2, err := Open(ctx, testDSN())
	if err != nil {
		t.Fatalf("second Open() error = %v, want nil", err)
	}
	defer st2.Close()

	_ = st // keep newTestStore's cleanup registered
}
