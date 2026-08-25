# Learning Go by Building Ambud

This is not a Go course. It's a map from Go concepts to the exact
moment in the [roadmap](ROADMAP.md) where Ambud needs them, so you learn
each one right before using it for real, instead of front-loading months
of tutorials before writing a line of Ambud code.

For each concept: what to learn, why *Ambud specifically* needs it,
where it shows up in the codebase, and a tiny standalone exercise to do
first — small enough for an evening, deliberately disconnected from
Ambud so getting it wrong costs nothing.

Work through these roughly in order, each one just before the roadmap
phase that needs it. Don't do the exercise for Phase 5's concepts while
you're still on Phase 1.

This document is Go-only, deliberately. Per [`ARCHITECTURE.md`](ARCHITECTURE.md)
and [`ROADMAP.md`](ROADMAP.md), the Web UI is a required, first-class
part of the project from Phase 0 (scaffold) and Phase 3 (real screens)
onward — but it's TypeScript/React work running in parallel to the Go
track below, not a Go learning goal itself. If React/TypeScript is also
new to you, that's a separate, ordinary frontend learning curve; it
isn't covered here because it doesn't touch the "learn Go by building"
premise this document exists for.

---

## 1. Go fundamentals & tooling
**When:** Before Phase 0.

**Learn:** `go mod init`, package structure, `go build`/`go run`/`go
test`, `gofmt`, basic syntax (no generics or concurrency yet — just
variables, functions, structs, slices, maps, control flow).

**Why Ambud needs it:** Everything. This is the floor, not a phase-
specific concept.

**Where it appears:** The entire repo, starting with `cmd/ambudctl/main.go`
in Phase 0.

**Exercise before using it in Ambud:** Write a standalone `wordcount`
CLI (no dependencies) that reads a text file and prints word frequency,
sorted by count. Forces you through slices, maps, structs, and basic I/O
without any Ambud complexity in the way.

---

## 2. Structs, interfaces, and error handling
**When:** Phase 1.

**Learn:** How Go interfaces work (implicit satisfaction, not
`implements`), idiomatic error handling (`if err != nil`, wrapping with
`fmt.Errorf("...: %w", err)`, `errors.Is`/`errors.As`) — this is
probably the single biggest mental shift from most other backend
languages' exception-based error handling.

**Why Ambud needs it:** The `internal/runtime` interface
(`Pull`/`Run`/`Stop`/`List`) from [`ARCHITECTURE.md`](ARCHITECTURE.md) is
Ambud's first real interface, and it exists specifically so containerd
calls can fail in many ways (image not found, daemon unreachable, name
collision) that all need distinct, well-wrapped errors bubbling up to
the CLI as useful messages, not stack traces.

**Where it appears:** `internal/runtime` (Phase 1), and this pattern
repeats for every interface Ambud defines afterward (`Scheduler` in
Phase 5, `store` methods in Phase 3).

**Exercise before using it in Ambud:** Define a `Shape` interface
(`Area() float64`, `Perimeter() float64`) with two implementations
(`Rectangle`, `Circle`), plus a function that returns an error for a
shape with a negative dimension, wrapped with context, and a caller
that unwraps it with `errors.Is`. Small, but it's every pattern
`internal/runtime` will use.

---

## 3. Building a CLI with Cobra
**When:** Phase 1.

**Learn:** `spf13/cobra` basics — commands, subcommands, flags, and how
it structures a multi-command CLI (`ambudctl run`, `ambudctl ps`,
`ambudctl stop` as siblings under one root command).

**Why Ambud needs it:** `ambudctl` is a real multi-command CLI from day
one, and Cobra is the de facto standard (it's what `kubectl`, `docker
compose`, and `gh` are built with) — worth learning the standard tool
rather than hand-rolling flag parsing.

**Where it appears:** `cmd/ambudctl`, every phase from Phase 1 onward as
new subcommands are added (`node`, `deploy`, `volume`, `login`, ...).

**Exercise before using it in Ambud:** A `taskcli` toy — `add "buy
milk"`, `list`, `done <id>` — backed by a JSON file on disk. Forces you
through Cobra's command tree and flag binding without touching
containerd or HTTP.

---

## 4. Goroutines, channels, and `context.Context`
**When:** Phase 2.

**Learn:** What a goroutine actually is (not an OS thread), `go func()`,
unbuffered vs. buffered channels, `select`, and `context.Context` for
cancellation/timeouts/deadlines — the mechanism nearly every Go HTTP
and I/O API uses to propagate "stop now."

**Why Ambud needs it:** The agent (Phase 2) is Ambud's first genuinely
concurrent program: it must serve HTTP requests *and* run a background
resource-collection loop *and* (from Phase 3) run a heartbeat loop, all
at once, without blocking each other. `context.Context` shows up in
literally every function signature that does I/O from this point
forward (`Run(ctx context.Context, ...)`), because containerd calls and
HTTP requests both need to be cancellable/timeout-bounded.

**Where it appears:** `internal/agent`'s background loops (Phase 2),
every `internal/runtime` and `store` method signature from here on.

**Exercise before using it in Ambud:** Write a small program that
launches 3 goroutines each simulating "work" (`time.Sleep`), reporting
results on a channel, with a `context.WithTimeout` that cancels
whichever goroutines haven't finished in time. This is structurally
identical to "run several background loops and be able to shut them all
down cleanly," which is exactly what the agent's `main()` needs to do
on `SIGTERM`.

---

## 5. `net/http`: building a REST API
**When:** Phase 2 (agent's local API), reinforced in Phase 3
(control plane's API).

**Learn:** `http.Handler`/`http.HandlerFunc`, routing (stdlib
`http.ServeMux` with Go 1.22+ pattern matching, or a light router like
`chi` if you want named path params sooner), middleware (a function that
wraps a handler — this is how request logging and, later, auth get
layered on), JSON encode/decode via `encoding/json`.

**Why Ambud needs it:** This is the transport for almost every
component boundary in [`ARCHITECTURE.md`](ARCHITECTURE.md) — CLI↔agent,
CLI↔control plane, agent↔control plane. Understanding it well here pays
off for the rest of the project.

**Where it appears:** `internal/agent`'s HTTP server (Phase 2),
`internal/controlplane/api` (Phase 3 onward).

**Exercise before using it in Ambud:** A tiny in-memory REST API for a
"notes" resource — `POST /notes`, `GET /notes`, `GET /notes/{id}`,
`DELETE /notes/{id}` — backed by a `map[string]Note` guarded by a
`sync.Mutex` (see #7 below). This is structurally the same shape as the
agent's container-lifecycle endpoints in Phase 2.

---

## 6. HTTP clients & designing a request/response contract
**When:** Phase 2 (`ambudctl` becomes an HTTP client).

**Learn:** `net/http.Client`, building requests, handling non-2xx
responses as errors, timeouts on the client side, and — the design
skill, not just the API — thinking about what a *response* should look
like from the caller's side before writing the handler.

**Why Ambud needs it:** `internal/apiclient` (see
[`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md)) is what `ambudctl`
becomes built on in Phase 2, and stays the shape of every future client
interaction with the control plane.

**Where it appears:** `internal/apiclient`, from Phase 2 onward.

**Exercise before using it in Ambud:** Point a small Go program at a
public JSON API (e.g., a free REST test API) and write a typed client
for 2–3 endpoints, including handling a 404 and a 500 distinctly. Small,
but "handle the failure cases distinctly" is the actual skill being
practiced.

---

## 7. `sync` primitives (Mutex, WaitGroup) and safe concurrent state
**When:** Phase 3–4 (control plane tracking multiple nodes at once).

**Learn:** `sync.Mutex`/`RWMutex` for protecting shared state accessed
by concurrent goroutines, `sync.WaitGroup` for waiting on a group of
goroutines to finish, and how to reason about what needs protecting
(anything read/written from more than one goroutine) vs. what doesn't.

**Why Ambud needs it:** Once the control plane is fielding heartbeats
from multiple nodes concurrently (Phase 4), any in-memory state it
keeps outside the database (e.g., a cache of "last heartbeat time" used
for fast online/offline checks) is being read and written from
multiple request-handling goroutines simultaneously. Get this wrong and
you get a data race — Go's race detector (`go test -race`) will catch
it if you know to run it.

**Where it appears:** Any in-memory cache in `internal/controlplane`;
also a good moment to introduce `go test -race` into CI.

**Exercise before using it in Ambud:** A concurrent counter — 100
goroutines each incrementing a shared counter 1000 times — first
without a mutex (run with `-race`, watch it fail/report a race), then
fixed with a `sync.Mutex`. This single exercise makes the failure mode
visceral before it matters in real code.

---

## 8. `database/sql` (or `pgx`), migrations, and the repository pattern
**When:** Phase 3.

**Learn:** Connecting to Postgres from Go, prepared statements /
parameterized queries (never string-concatenate SQL — this is also a
security requirement, not just style), scanning rows into structs, a
migration tool (`golang-migrate` or `goose`), and structuring a `store`
package so SQL lives in one place behind typed methods.

**Why Ambud needs it:** PostgreSQL is the control plane's entire system
of record (see [`ARCHITECTURE.md`](ARCHITECTURE.md)) — nodes,
workloads, and later users all live here.

**Where it appears:** `internal/controlplane/store` and
`store/migrations`, from Phase 3 onward.

**Exercise before using it in Ambud:** A standalone `bookmarks` CLI
backed by a local Postgres (or SQLite, if you want zero setup for the
exercise) with a migration creating the table and a `store` package
exposing `Create`, `List`, `Delete` — same shape as
`internal/controlplane/store` will need for `nodes` and `workloads`.

---

## 9. Table-driven tests and testing HTTP handlers
**When:** Ongoing from Phase 1, formalized by Phase 2–3.

**Learn:** Go's table-driven test idiom (a slice of input/expected-output
cases run through one test function), `httptest.NewRecorder`/
`httptest.NewServer` for testing HTTP handlers without a real network
socket, and designing code so it *can* be tested without a live
containerd or Postgres (dependency injection via interfaces — ties back
to #2).

**Why Ambud needs it:** CI (Phase 0) has no containerd and no Postgres
available by default — the only way `internal/agent` and
`internal/controlplane` get meaningful test coverage in CI is if their
core logic is written against interfaces (`runtime.Runtime`,
`store.Store`) that can be swapped for an in-memory fake in tests.

**Where it appears:** Every `_test.go` file from Phase 1 onward; this is
as much a design discipline (interfaces at the right seams) as a testing
technique.

**Exercise before using it in Ambud:** Take the "notes" API from
exercise #5 and write table-driven tests for every endpoint using
`httptest`, including at least one error case (bad JSON body, missing
ID) per endpoint.

---

## 10. Structured logging (`log/slog`)
**When:** Phase 2, reinforced through Phase 10.

**Learn:** `log/slog` (Go's standard structured logging since 1.21) —
leveled logs, structured key-value fields instead of formatted strings,
attaching context (request ID, node ID) to a logger and passing it
through.

**Why Ambud needs it:** A distributed system with multiple processes is
unpleasant to debug from `fmt.Println` output. Structured logs (`node_id
=abc123 event=heartbeat_received`) are what make Phase 4+ debugging
("why did node 2 go offline at 3am") tractable.

**Where it appears:** Both binaries, from Phase 2 onward; explicitly
audited in Phase 10 to ensure no secrets (tokens, passwords) get logged.

**Exercise before using it in Ambud:** Rewrite the "notes" API's
`fmt.Println` debug output as `slog` calls with proper levels
(`Info`/`Warn`/`Error`) and structured fields, and configure JSON output
mode (useful for later log aggregation, even though Ambud doesn't build
a log pipeline itself — see Phase 8's "not building" list).

---

## 11. Reading system resources (`/proc`, or `gopsutil`)
**When:** Phase 2.

**Learn:** Either reading `/proc/meminfo`, `/proc/stat`, and
`/proc/diskstats` directly (more educational, more Linux-specific — fits
Ambud's Linux-first target) or using `github.com/shirou/gopsutil` for a
cross-platform abstraction (useful since your dev machine may not be
Linux — see [`DEVELOPMENT.md`](DEVELOPMENT.md)).

**Why Ambud needs it:** The agent's `GET /resources` endpoint and every
heartbeat depend on this — it's the data the scheduler (Phase 5)
ultimately places workloads on.

**Where it appears:** `internal/agent`'s resource-collection loop
(Phase 2).

**Exercise before using it in Ambud:** A standalone `sysinfo` CLI that
prints current CPU count, memory used/total, disk used/total, refreshed
every 2 seconds (combining this with goroutines/`select` from #4 for the
refresh loop).

---

## 12. Designing and testing an interface with multiple call sites
**When:** Phase 5.

**Learn:** How to design a Go interface for *testability and future
substitution* rather than just "because OOP" — the `Scheduler` interface
in [`ARCHITECTURE.md`](ARCHITECTURE.md) is deliberately narrow (one
method) so it's trivial to write a fake for handler tests and trivial to
add a second implementation later without touching callers.

**Why Ambud needs it:** This is where the "interfaces at the right
seams" discipline from #2 and #9 pays off concretely — the scheduler
needs to be unit-testable against a fixed set of fake nodes without a
real cluster.

**Where it appears:** `internal/controlplane/scheduler` (Phase 5).

**Exercise before using it in Ambud:** No new exercise needed — this is
the moment to apply #2 and #9 directly to a real Ambud package for the
first time on your own, before referring back to this doc.

---

## 13. gRPC and Protocol Buffers (optional, stretch)
**When:** Phase 8, only if pursuing the "low-latency push" stretch goal.

**Learn:** `.proto` file syntax, code generation (`protoc` +
`protoc-gen-go`/`protoc-gen-go-grpc`), unary vs. streaming RPCs, and how
a long-lived bidirectional stream differs from request/response REST.

**Why Ambud needs it:** REST polling (Phases 3–7) is simple and correct
but has heartbeat-interval latency. A long-lived gRPC stream between
agent and control plane is the natural upgrade if that latency ever
matters in practice — and it's a good excuse to learn gRPC on a real,
already-working system instead of a tutorial, per the project brief's
"REST initially, potentially gRPC where useful."

**Where it appears:** Optionally, an alternate transport in `internal/
agent` and `internal/controlplane/api`, additive to (not replacing) the
REST API used by the CLI and Web UI.

**Exercise before using it in Ambud:** A tiny two-service gRPC example
— a "ping stream" where a client opens a stream and the server pushes a
timestamp every second — before wiring anything into the agent.

---

## 14. TLS and mTLS in Go
**When:** Phase 10.

**Learn:** `crypto/tls`, generating a self-signed CA and leaf certs for
LAN use, configuring `http.Server`/`http.Client` for TLS, and — if
pursuing mTLS for agent connections — client certificate verification.

**Why Ambud needs it:** Production hardening (Phase 10) requires
encrypting control-plane traffic and, ideally, moving agent
authentication from a bearer token to a client certificate.

**Where it appears:** Both binaries' HTTP server/client setup, `deploy/`
scripts for generating dev certs.

**Exercise before using it in Ambud:** Generate a self-signed cert with
`openssl` (or Go's own `crypto/x509` cert-generation APIs, for extra
practice), stand up a minimal `net/http` server using it, and confirm a
Go client rejects it without a trusted CA pool and accepts it with one.

---

## Concepts intentionally left off this list

Generics, reflection, and build constraints/`cgo` are all real Go
features, but nothing in the roadmap through Phase 10 requires them.
Learn them if and when a concrete need shows up (e.g., generics might
simplify a repeated store-layer pattern eventually) — not preemptively.
This list stays tied to what Ambud actually needs, per the "no generic
30-day Go course" brief.
