# Project Structure

This document proposes a repo layout for Ambud and, more importantly,
explains the reasoning — this is a monorepo housing three Go binaries, a
TypeScript web app, and deployment artifacts, and the layout should
reflect the actual dependency structure between them, not a template
copied from elsewhere.

## Proposed layout

```
ambud/
├── cmd/
│   ├── ambudctl/              # CLI binary — main package only, thin
│   ├── ambud-agent/           # node agent binary — main package only, thin
│   └── ambud-controlplane/    # control plane binary — main package only, thin
├── internal/
│   ├── agent/                 # agent business logic: reconcile loop,
│   │                          #   resource collection, heartbeat client
│   ├── controlplane/
│   │   ├── api/                # HTTP handlers, request/response types, routing
│   │   ├── scheduler/          # Scheduler interface + implementation(s)
│   │   ├── store/               # Postgres access layer
│   │   └── store/migrations/   # SQL migration files
│   ├── runtime/                # containerd wrapper (used by internal/agent)
│   ├── apiclient/              # shared HTTP client used by ambudctl and
│   │                          #   (later) any other Go client of the API
│   └── apitypes/               # request/response structs shared by
│                              #   control plane, agent, and ambudctl
├── api/
│   └── openapi.yaml            # REST contract — source of truth for both
│                              #   the Go server/client and the TS web client
├── web/                        # React + TypeScript dashboard — own
│   ├── package.json           #   package.json, own toolchain, generates
│   ├── src/                   #   its API types from api/openapi.yaml
│   └── ...
├── deploy/
│   ├── systemd/                # .service unit files for agent & control plane
│   ├── compose/                # docker-compose for local Postgres, etc.
│   └── scripts/                # install/uninstall helper scripts
├── scripts/                    # dev-only scripts (cross-compile, lint, etc.)
├── docs/                       # this documentation
├── go.mod / go.sum
├── Makefile
└── README.md
```

## Reasoning, decision by decision

### `cmd/` + `internal/`, no `pkg/`

The classic "standard Go project layout" includes a `pkg/` directory for
code meant to be imported by other projects. Ambud doesn't have external
importers yet — nobody outside this repo depends on Ambud's Go packages,
and there's no concrete plan for an external Go SDK in the near term.
Adding `pkg/` now would be guessing at a future need instead of
responding to a real one (see the project's own "don't design for
hypothetical future requirements" principle).

`internal/` does real work here, not just convention: the Go compiler
*enforces* that nothing outside this module can import
`internal/runtime` or `internal/controlplane/store`. That's exactly the
right default for code that isn't a stable, intentional public API yet.
If Ambud later ships an official Go client library, it gets promoted
out of `internal/` deliberately, at that point, as its own package (or
its own module) — not scaffolded speculatively today.

`cmd/` holds three thin `main.go` files whose only job is argument
parsing and wiring — constructing the real dependencies (store, runtime
client, scheduler) and handing off to `internal/` packages that contain
all the actual logic and are unit-testable without a `main()` in the
loop.

### `internal/apitypes` and `internal/apiclient` as their own packages

The request/response structs that flow over the control-plane REST API
are used by three consumers: the control plane (to encode responses),
the agent (to decode heartbeat responses / encode reports), and
`ambudctl` (to encode requests / decode responses). Putting them in
their own package avoids a situation where `internal/agent` has to
import `internal/controlplane/api` just to get a struct definition,
which would wrongly couple the agent to control-plane internals.
`apiclient` similarly exists so `ambudctl`'s `cmd/` package doesn't
reimplement HTTP plumbing, and so a future second Go-based client
(imagine a Terraform provider) has something to import.

### `api/openapi.yaml` as the source of truth for the contract

This lives outside `internal/` on purpose: it's the one artifact that
both the Go side and the TypeScript web UI need to agree on, and it
should be readable/diffable independent of either language's tooling.
The intent is that `internal/apitypes` (Go) and `web/src/api/` (TS,
generated) both derive from this file, rather than the two languages'
type definitions drifting out of sync by hand. Early phases can write
`openapi.yaml` by hand or even defer it slightly and hand-write types on
both sides — but the directory exists from Phase 0 so there's an obvious
home for the contract once it's formalized, rather than it accidentally
living inside `internal/controlplane/api` where the web project can't
reasonably reach it.

If/when gRPC is introduced (see [`ROADMAP.md`](ROADMAP.md) Phase 8),
`.proto` files also live under `api/` for the same reason — language-
agnostic contracts belong together, separate from any one consumer's
internal package tree.

### `internal/controlplane/*` subpackages

`api`, `scheduler`, and `store` are separate packages *within* one
binary (`ambud-controlplane`), not separate binaries. This mirrors the
architectural decision documented in
[`ARCHITECTURE.md`](ARCHITECTURE.md) and [ADR 0001](ADR/0001-initial-architecture.md):
one control-plane process is enough for the scale Ambud targets, and
splitting into microservices prematurely would add deployment and
operational complexity (service discovery between control-plane
components, more processes to keep alive) with no present benefit. The
package boundaries exist so that split is *possible later without a
rewrite* — `api` depends on `scheduler` and `store` through interfaces,
not the reverse, so any one of them could become its own process and
talk over gRPC instead of a Go function call, if that's ever justified
by actual load.

### `store/migrations/` colocated with `store/`

Migrations are Postgres-schema code, not general "data files" — keeping
them next to the Go package that runs them (rather than a top-level
`/migrations`) keeps the coupling between schema and the queries that
depend on it visible in one place while browsing the code.

### `web/` as a fully separate root-level project

The web dashboard has its own `package.json`, `node_modules`, linting,
and build tooling — none of that should live inside or leak into the Go
module. Keeping it at the repo root (sibling to `cmd/`, `internal/`)
rather than nested inside, say, `cmd/ambud-controlplane/web/` keeps `go
build ./...` and `npm install` cleanly independent operations, and makes
it obvious to a new contributor that this is "the other half" of the
project — per [`ARCHITECTURE.md`](ARCHITECTURE.md), the Web UI is the
primary way an operator uses Ambud, not a secondary add-on to one
binary. It's scaffolded here from [Phase 0](ROADMAP.md#phase-0--project-setup)
of the roadmap, even before it has real screens, so its build/lint/CI
story is established alongside the Go tooling from commit one rather
than bolted on later.

### `deploy/` separate from `docs/`

`deploy/` is *executable or runnable* artifacts (systemd units, compose
files, install scripts) — "how Ambud actually gets run." `docs/` is
prose. Mixing them (e.g., a systemd unit example embedded only in a
markdown file) makes the unit file impossible to reference directly from
an install script. `DEVELOPMENT.md` and the roadmap phases reference
`deploy/` paths directly for exactly this reason.

### Single Go module, not a multi-module workspace

All three `cmd/` binaries and all `internal/` packages live in one
`go.mod` at the repo root. A multi-module `go.work` setup (separate
`go.mod` per binary) is the alternative, and is worth reconsidering only
if/when independent versioning or independent release cadences between
the agent and control plane become a real need — for a project at this
stage, a single module means one `go build ./...`, one `go test ./...`,
one dependency set to keep updated, and no cross-module version
juggling. This is the more boring choice, deliberately.

## What to avoid

- Don't create `pkg/` "just in case" — see above.
- Don't let `cmd/*/main.go` grow real logic — if a `main.go` is doing
  anything beyond flag parsing and constructing/wiring dependencies,
  that logic belongs in `internal/`, where it can be unit tested without
  invoking the binary.
- Don't let `internal/agent` or `internal/controlplane/*` import each
  other directly — they should only share `internal/apitypes`. If the
  agent ever needs something from control-plane code, that's a sign the
  shared concept belongs in `apitypes` (or a new shared package), not
  that the boundary should be crossed.
- Don't scatter SQL queries as inline strings deep inside handler code —
  keep them in `internal/controlplane/store`, with `api` calling typed
  methods (`store.GetNode(ctx, id)`), so the SQL surface is auditable in
  one place.

This structure is expected to hold steady through roughly Phase 8 of the
[roadmap](ROADMAP.md). If a component is ever split into its own
process (the API server separated from the scheduler, for instance) or
promoted to its own module, that's worth its own ADR at the time,
following the pattern in [`ADR/0001-initial-architecture.md`](ADR/0001-initial-architecture.md).
