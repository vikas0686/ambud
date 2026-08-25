# Ambud Roadmap

This roadmap breaks Ambud into 11 phases, from an empty repo to a
hardened multi-node cluster. It's written for a **solo developer learning
Go while building**, so phases are scoped to be completable in evenings/
weekends over a few weeks each, not sprints. Treat phase boundaries as
commit-series boundaries, not calendar deadlines.

Two rules hold across every phase:

1. **Every phase ends with something you can actually run.** No phase is
   "just refactoring" or "just design" — there's always a demo at the end.
2. **Do not start a phase's "should not build yet" list.** It's there to
   stop scope creep at the exact moment it's most tempting (see
   [ADR 0001](ADR/0001-initial-architecture.md) for why each exclusion
   was chosen).

The **MVP** (see [`MVP.md`](MVP.md)) is reached at the end of **Phase 5**.
Everything before that is building toward it in a straight line;
everything after it is hardening and expanding a system that already
works end-to-end.

Related reading: [`ARCHITECTURE.md`](ARCHITECTURE.md) for what each
component listed below actually does, [`GO_LEARNING_PATH.md`](GO_LEARNING_PATH.md)
for the Go concepts each phase is a natural place to learn, and
[`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md) for where code in each
phase should live.

---

## Phase 0 — Project setup

**Goal.** A repo that builds, tests, and lints in CI, with nothing
Ambud-specific in it yet.

**What we're building.** Go module skeleton, `Makefile`, GitHub Actions
CI (`go build`, `go vet`, `go test`, `golangci-lint`), `.gitignore`,
license file, the `docs/` you're reading now, and an empty **Web UI
scaffold** (`web/`, a bare Vite + React + TypeScript app that builds and
runs, showing a placeholder page — no real screens yet).

**Why it matters.** Every phase after this assumes `make test` and CI
exist. Setting them up after the fact is always more painful than
setting them up first, and for a Go beginner, a working lint/test loop
from commit one catches mistakes immediately instead of silently.
Scaffolding `web/` here too — even though it does nothing yet — is
deliberate: per [`ARCHITECTURE.md`](ARCHITECTURE.md), the Web UI is a
required, first-class client, not something bolted on after the "real"
backend work is done. Having the empty shell exist from commit one keeps
its build/lint/CI story established alongside the Go tooling, instead of
introduced as an afterthought once someone remembers it's needed.

**Technologies.** Go toolchain, GitHub Actions, `golangci-lint`, `make`,
Node.js/TypeScript, Vite, React.

**Implementation tasks.**
- `go mod init github.com/<you>/ambud`
- Empty `cmd/ambudctl/main.go` that prints a version string
- `Makefile` with `build`, `test`, `lint`, `fmt` targets
- `.github/workflows/ci.yml` running build + vet + test + lint on push/PR
- `README.md` (already drafted — commit it), `LICENSE`, `.gitignore`
- Pre-commit or CI-enforced `gofmt -l` check
- `web/`: `npm create vite@latest` (React + TS template), a placeholder
  page, `npm run build`/`npm run lint` wired into CI as a separate job
  from the Go one

**Expected outcome.** `git clone`, `make build`, `make test` all work
on a clean checkout; `cd web && npm install && npm run dev` serves a
placeholder page; CI is green on the first PR for both the Go and web
jobs.

**Do NOT build yet.** Anything Ambud-specific — no agent, no API, no
containerd calls, no real UI screens (there's no API yet for the UI to
call — see Phase 1's note on this). Not even a real CLI command beyond
`version`.

**Definition of done.** CI pipeline green on `main` (Go build/test/lint
+ web build/lint as separate jobs), `make build` produces a binary,
`web/` builds and serves a placeholder page, `docs/` committed.

---

## Phase 1 — Single-node prototype

**Goal.** Prove the core mechanic — "Go code drives containerd" — with
the smallest possible program, before any client/server split exists.

**What we're building.** A single CLI (`ambudctl`, or a temporary
`ambud-local` binary) that talks directly to the local containerd socket
to pull an image and run/stop a container. No control plane, no agent,
no network calls at all — this is one process, one machine, run by
hand.

**Why it matters.** This is where you learn the containerd Go client and
basic Go program structure without also juggling HTTP, databases, and
multi-process coordination. Getting `ctr run` -equivalent behavior
working in your own Go code is the foundational skill everything else
(the agent's reconcile loop) builds on. It's also the fastest possible
path to "I built something real," which matters for momentum.

**Technologies.** Go, `github.com/containerd/containerd` client library,
`spf13/cobra` for CLI structure, containerd itself (installed on your
Linux dev box/VM — see [`DEVELOPMENT.md`](DEVELOPMENT.md)).

**Implementation tasks.**
- Install containerd on your Linux dev environment; confirm `ctr` works
  manually first, so you know the baseline before writing Go against it
- `internal/runtime` package: `Pull(ctx, image string) error`,
  `Run(ctx, name, image string) error`, `Stop(ctx, name string) error`,
  `List(ctx) ([]ContainerStatus, error)` — wrapping the containerd client
  behind Ambud's own small interface (see [`ARCHITECTURE.md`](ARCHITECTURE.md))
- `ambudctl run <image>`, `ambudctl ps`, `ambudctl stop <name>` calling
  that package directly (in-process, no HTTP yet)
- Basic error handling: what happens if the image doesn't exist, if
  containerd isn't running, if the container name collides
- Table-driven unit tests for anything that doesn't require a live
  containerd (e.g., name validation, status formatting)

**Expected outcome.** From a Linux box with containerd running:
`ambudctl run docker.io/library/nginx:alpine` actually pulls and starts
nginx; `ambudctl ps` shows it; `ambudctl stop nginx` stops it.

**Do NOT build yet.** No HTTP server, no agent process, no control
plane, no database, no multi-node anything. No systemd unit yet — run it
by hand from a terminal. **No Web UI work this phase** — this is the one
deliberate exception to "UI is first-class from the start": there is no
network API for a UI to call until Phase 2, so there is nothing
meaningful for it to do yet. The `web/` scaffold from Phase 0 sits idle;
that's expected, not a gap.

**Definition of done.** You can run, list, and stop a real container via
your own Go binary, on real containerd, with tests passing in CI (CI
won't have containerd — structure code so runtime-dependent logic is
isolated and testable without a live daemon).

---

## Phase 2 — Node agent

**Goal.** Turn the Phase 1 logic into a long-running service with its
own local HTTP API — still one machine, but now shaped like the
component that will later run on every node.

**What we're building.** `ambud-agent`: a daemon that exposes a local
REST API (`POST /containers`, `GET /containers`, `POST /containers/{id}/stop`,
etc.) wrapping the same `internal/runtime` package from Phase 1.
`ambudctl` becomes an HTTP client of this API instead of calling
containerd in-process.

**Why it matters.** This is the architectural seam that makes multi-node
possible later: once the CLI talks to the agent over HTTP instead of
in-process, "the agent is on a different machine" is a one-line config
change (a different URL), not a redesign. It's also where goroutines,
channels, and `net/http` enter the codebase for real.

**Technologies.** Go `net/http` (or a light router like `chi`),
goroutines for background work (e.g., a periodic resource-collection
loop), `context.Context` for request cancellation/timeouts.

**Implementation tasks.**
- `cmd/ambud-agent/main.go`: starts an HTTP server, wires it to
  `internal/runtime`
- REST endpoints for container lifecycle (create/list/get/stop/restart)
  and a `GET /resources` endpoint returning CPU/RAM/disk facts (using
  `gopsutil` or reading `/proc` directly — good small Go exercise either
  way)
- `ambudctl` rewritten to be a pure HTTP client (`internal/apiclient` or
  similar), configurable agent URL
- A background goroutine in the agent that refreshes cached resource
  facts every few seconds, so `GET /resources` doesn't block on syscalls
  per request
- systemd unit file for running the agent as a real daemon (`deploy/systemd/ambud-agent.service`)
- Structured logging (`log/slog`) instead of `fmt.Println`

**Expected outcome.** `systemctl start ambud-agent`, then from anywhere
on the same machine (or LAN, if bound to `0.0.0.0`), `ambudctl --agent
http://localhost:8080 run nginx` works exactly like Phase 1, but over
HTTP.

**Do NOT build yet.** No control plane, no database, no node
registration/heartbeat (there's no one to heartbeat to yet), no auth on
the agent API — it's still trusted-LAN-only at this point. **Still no
Web UI screens** — the agent's local API exists here, but the UI's
intended backend is always the control plane's cluster-level API
(Phase 3+), never a single agent directly, so there's still nothing for
it to sensibly attach to yet.

**Definition of done.** Agent runs as a systemd service, survives
`ambudctl` disconnecting, exposes working REST endpoints for container
lifecycle + resource facts, covered by tests that hit the HTTP handlers
with a fake runtime implementation (no real containerd needed for these
tests).

---

## Phase 3 — Control plane

**Goal.** Introduce the control plane as a genuinely separate process
with its own database, and make the Phase 2 agent register with and
report to it — still just one node, but now the real two-process shape
exists.

**What we're building.** `ambud-controlplane`: a new binary with its own
REST API, backed by PostgreSQL, that knows about nodes and workloads.
The agent is modified to register itself on startup and heartbeat
periodically. The CLI switches to talking to the control plane instead
of the agent directly for anything cluster-level (`ambudctl node list`),
while still being able to reach an agent directly for low-level debug
use if useful. **The Web UI gets its first real screens here**: a node
list page and a deploy form, calling the exact same control-plane REST
API the CLI now uses — this is the earliest point a UI has anything
meaningful to show, so it's the earliest point it's required to exist.

**Why it matters.** This is the architectural core of the whole project
— the control-plane/data-plane split described in
[`ARCHITECTURE.md`](ARCHITECTURE.md). Getting it right with one node is
much easier than debugging it for the first time with three. This phase
is also where `database/sql`, migrations, and designing a REST API for
someone other than yourself all enter the picture.

**Technologies.** PostgreSQL, a migration tool (`golang-migrate` or
`pressly/goose`), `pgx` or `database/sql` + `lib/pq`/`pgx` driver,
`net/http`, join-token-based auth for the agent-to-control-plane leg.

**Implementation tasks.**
- Postgres schema + migrations: `nodes`, `workloads` tables (see
  [`ARCHITECTURE.md`](ARCHITECTURE.md) for what each stores)
- `POST /v1/nodes/register` (join token → node record + credential),
  `POST /v1/nodes/{id}/heartbeat` (resources + container statuses in,
  desired-state delta out)
- Agent: on startup, register if no stored credential exists; otherwise
  heartbeat on a fixed interval with current resource facts + local
  reconcile status
- Agent's reconcile loop: instead of `ambudctl` calling the runtime
  directly, the agent now reconciles its containerd state toward
  whatever desired state the control plane's heartbeat response says —
  this is the shift from "agent as dumb executor of direct commands" to
  "agent as a local controller reconciling toward desired state"
- `ambudctl node list`, `ambudctl deploy <spec>` hitting the control
  plane's API
- Simple bearer-token join flow: operator runs
  `ambudctl node generate-join-token`, pastes it into the agent's config
  on the node being added
- Basic structured logging + request logging middleware on both
  processes
- Web UI: a node list page (polling `GET /v1/nodes`, showing name,
  status, live resource facts) and a deploy form (`POST /v1/workloads`)
  — both built against the OpenAPI contract from
  [`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md), with generated
  TypeScript types so the UI can't silently drift from what the API
  actually returns

**Expected outcome.** Start Postgres, start `ambud-controlplane`, start
`ambud-agent` with a join token pointed at it, run `ambudctl node list`
and see the one node with live resource facts. `ambudctl deploy` creates
a workload row; on the next heartbeat, the agent picks it up and starts
the container; `ambudctl node list` (or a status command) reflects it
running. **The same is true from the Web UI**: loading it shows the node
and its live resources, and submitting the deploy form results in the
container running — no capability introduced this phase is CLI-only.

**Do NOT build yet.** No second node, no scheduler (there's only one
node, so "placement" is trivial/unnecessary — resist building it early),
no auth for human users (join tokens are for nodes, not people — user
auth is Phase 9), no TLS yet (document it as a known gap, see Phase 10).
No UI design system or visual polish yet — plain, functional HTML/forms
are correct at this stage; see the note at the end of this roadmap on
where dedicated UI/UX work fits.

**Definition of done.** Two independently-running processes
(control plane, agent) plus Postgres, coordinating over HTTP, survive a
control-plane restart without losing track of the node, and a deploy
issued via `ambudctl` **or the Web UI** actually results in a running
container — all demonstrable on one machine (agent and control plane can
run on the same box for this phase; separating them is just a config/
network change, proven in Phase 4).

---

## Phase 4 — Multi-node cluster

**Goal.** Add a second physical machine and prove nothing in the design
so far was secretly single-node-only.

**What we're building.** Nothing new architecturally — this phase is
almost entirely about *validating* Phases 2–3 against a second real
machine, plus the config/UX needed to make joining a node a smooth
operator experience.

**Why it matters.** This is the phase that turns "a client-server toy"
into "a cluster." It's also where hidden single-node assumptions
(hardcoded `localhost`, unique-name collisions, port conflicts) get
caught — cheaply, with two nodes, instead of expensively, with ten.

**Technologies.** Same as Phase 3; possibly your repurposed old Windows
PC re-installed with Linux as node #2 (see
[`DEVELOPMENT.md`](DEVELOPMENT.md)) — a nice moment where the project's
own premise becomes literally true for you.

**Implementation tasks.**
- Fix any node-identity assumptions that implicitly assumed one node
  (container name collisions across nodes, node ID generation, etc.)
- Confirm the join-token flow works cleanly for a node on a *different*
  physical machine, across a real LAN, not just `localhost`
- `ambudctl node list` output needs to be genuinely useful with 2+ rows:
  clear columns for name, status (online/offline via heartbeat timeout),
  resources
- Heartbeat-timeout logic in the control plane: mark a node "unreachable"
  after N missed heartbeats, surfaced in `node list`
- Manual test plan: kill node 2's agent, confirm control plane marks it
  offline but node 1 is unaffected; restart node 2's agent, confirm it
  rejoins using its stored credential (not a fresh join token)
- Web UI: node list page now genuinely needs to handle 2+ rows well —
  online/offline badges, per-node resource bars; confirm the deploy form
  lets you pick a target node explicitly (the "no node specified"
  scheduler experience is Phase 5)

**Expected outcome.** Two machines, each running `ambud-agent`, both
visible and correctly labeled online/offline in `ambudctl node list`
**and in the Web UI's node list page** against one shared control plane.
Deploys still land on whichever node you deploy directly to (no
scheduler yet — see Phase 5) or on a hardcoded/default node.

**Do NOT build yet.** Still no real scheduler (deploy can target a
specific node ID explicitly for now), no cross-node networking (Phase
6), no shared storage (Phase 7).

**Definition of done.** Two independent physical machines, both joined
to one control plane, both correctly reflecting online/offline status,
with a documented manual test proving node failure is isolated (this is
the moment the MVP's "support adding a second machine" requirement is
met — see [`MVP.md`](MVP.md)).

---

## Phase 5 — Container scheduling

**Goal.** Remove the requirement that a deploy names a specific node —
let the control plane pick.

**What we're building.** The `Scheduler` interface described in
[`ARCHITECTURE.md`](ARCHITECTURE.md) and its first real implementation:
filter nodes with enough free resources, then pick the one with the most
free RAM.

**Why it matters.** This is the last piece needed to call Ambud an MVP
(see [`MVP.md`](MVP.md)) — "deploy a workload to one of the available
machines" without the operator manually tracking capacity in their head.
It's a small amount of code, but it's the conceptual heart of what makes
this a *cluster* platform rather than "SSH to a specific box and run
docker."

**Technologies.** Pure Go — no new external dependency needed here.

**Implementation tasks.**
- `internal/controlplane/scheduler` package implementing the
  `Scheduler` interface from `ARCHITECTURE.md`
- Resource-requirement fields on the workload spec (CPU, RAM requested)
  — even if not enforced via cgroup limits yet, they're needed for
  placement math
- Wire the scheduler into the deploy API handler: `ambudctl deploy`
  no longer requires (though may still accept, as an override) a
  target node
- Failure path: no node has enough free resources → deploy request
  fails with a clear error, doesn't silently pick an overcommitted node
- Basic tests: given a fixed set of nodes with known resources, assert
  the scheduler picks the expected one, and correctly rejects when none
  fit
- Web UI reaches full functional parity with the CLI this phase: deploy
  form's node field becomes optional ("let the scheduler pick"), and
  workload rows gain start/stop/restart buttons and a status badge
  (running/exited/crash-looping) — this is the point where every one of
  the [10 MVP capabilities](MVP.md) must be completable by clicking
  alone, with no terminal open

**Expected outcome.** `ambudctl deploy nginx --cpu 1 --ram 512Mi` (no
node specified) lands on whichever of your nodes has room, and a second
identical deploy correctly lands on (or is rejected from, depending on
capacity) the right place. **The same deploy, done from the Web UI by
leaving the node field blank, behaves identically** — same scheduler,
same API, same result.

**Do NOT build yet.** No bin-packing optimization, no priority/
preemption, no affinity/anti-affinity, no taints/tolerations, no
autoscaling. "Most free RAM wins" is the entire algorithm. No UI
design-system pass yet — functional parity, not visual polish, is the
bar for this phase (see the roadmap's closing note on where that fits).

**Definition of done.** All 10 MVP capabilities from
[`MVP.md`](MVP.md) work end-to-end on 2+ real machines, **each one
completable both via `ambudctl` and via the Web UI** — this is the
explicit UI-parity bar the MVP is held to, not an optional stretch goal.
**This is the MVP milestone.**

---

## Phase 6 — Networking

**Goal.** Make containers reachable — first from the host, later
(optionally) from each other across nodes — without building a custom
overlay network.

**What we're building.** Host-port mapping (`nodeIP:hostPort` →
container port), exactly like `docker run -p`. A workload spec gains an
optional port-mapping field; the agent passes it through to containerd/
the OCI runtime spec.

**Why it matters.** A container nobody can reach isn't a useful
deployment. But cross-node container-to-container networking (an
overlay mesh) is one of the hardest problems in this whole space —
genuinely comparable in difficulty to everything else in the roadmap
combined. Ambud deliberately ships the 80%-useful, well-understood
solution (host ports) first, and treats the overlay network as optional
future work, not a blocker.

**Technologies.** containerd/OCI runtime spec port mapping (or a thin
iptables/nftables rule the agent manages, depending on how containerd's
networking is configured), no new major dependency for the host-port
case.

**Implementation tasks.**
- Extend workload spec: `ports: [{container: 80, host: 8080}]`
- Agent: apply the mapping when creating the container (via CNI bridge +
  port-forward rule, or containerd's built-in networking, depending on
  what Phase 1–2 set up — document the choice)
- Surface the reachable address (`nodeIP:hostPort`) in `ambudctl ps` /
  `node list` output, since the operator needs to know where to point a
  browser or client
- Guard against host-port collisions on the same node (two workloads
  can't claim the same host port) — surfaced as a scheduling constraint,
  not just a runtime failure
- **Stretch goal, not required for this phase:** a minimal name-based
  lookup (control plane records "workload X → nodeIP:port", `ambudctl
  resolve X` prints it) as a stepping stone toward real service
  discovery, without building DNS or an overlay network

**Expected outcome.** `ambudctl deploy nginx --port 80:8080` and then
`curl http://<node-ip>:8080` from another machine on the LAN actually
reaches the container, regardless of which node the scheduler picked.

**Do NOT build yet.** No overlay/mesh network, no CNI plugin
development, no service mesh, no cross-node container-to-container DNS.
If this is ever tackled, it should integrate an existing mature project
(e.g. a WireGuard-based mesh) rather than a custom protocol — see the
ADR.

**Definition of done.** A deployed container is reachable by IP:port
from outside its node, port collisions are prevented, and the
"where do I reach this" question always has a discoverable answer via
`ambudctl`.

---

## Phase 7 — Storage

**Goal.** Let a workload keep data across container restarts, on the
node it runs on.

**What we're building.** Node-local persistent volumes: a host directory
(e.g., `/var/lib/ambud/volumes/<volume-id>`) bind-mounted into the
container. The scheduler treats an existing volume as a hard placement
constraint (the workload must land on the node that has it).

**Why it matters.** Most real workloads (databases, anything with
state) are useless without persistence. This phase intentionally stops
at the simplest correct answer — see
[`ARCHITECTURE.md`](ARCHITECTURE.md)'s Storage Layer section — rather
than chasing distributed storage.

**Technologies.** Bind mounts via the OCI runtime spec / containerd
client, no new external dependency.

**Implementation tasks.**
- `volumes` table: volume ID, owning node, host path, created-at
- Workload spec: `volumes: [{name, containerPath}]`
- Volume creation: `ambudctl volume create <name>` — control plane picks
  (or requires specifying) a node, records the mapping, agent creates
  the directory on next reconcile
- Scheduler change: if a workload references an existing volume, the
  candidate node list collapses to just the volume's owning node (and
  the deploy fails clearly if that node lacks capacity, rather than
  silently placing elsewhere)
- `ambudctl volume list`, basic cleanup on volume deletion

**Expected outcome.** A Postgres or similar stateful container, deployed
with a volume, can be stopped, restarted, even have its container
recreated, and retain its data — as long as it stays pinned to its
node.

**Do NOT build yet.** No replication, no cross-node volume migration, no
snapshotting/backup automation, no distributed/network filesystem (NFS,
Ceph, etc.) integration. Explicitly permanently out of scope unless a
real need emerges — and if it does, integrate an existing project rather
than build one.

**Definition of done.** Data survives container restart and recreation
on the same node; the scheduler correctly refuses (rather than silently
mishandles) a volume-pinned deploy to a node that doesn't have the
volume.

---

## Phase 8 — Monitoring

**Goal.** See resource usage and workload health over time, not just as
a live snapshot — and make that data consumable by existing tools
instead of building a bespoke dashboard-only story.

**What we're building.** A `/metrics` endpoint (Prometheus exposition
format) on both the control plane and agents, plus **historical charts**
in the already-existing Web UI (resource usage over time, not just the
live snapshot it's shown since Phase 3) — Prometheus itself stays
optional/for-power-users, the built-in UI charts do not require it.

**Why it matters.** Phase 2 already collects raw resource facts for
scheduling, and the Web UI has shown live snapshots of them since Phase
3 — this phase is about (a) exposing that data to external tools
usefully via a standard format, and (b) finally showing *trends*, not
reinventing a time-series database when Prometheus already exists and
is exactly the right boring tool for that job.

**Technologies.** `github.com/prometheus/client_golang`, optionally
Grafana (external, operator's choice, not bundled), React charts (a
lightweight library, not a full BI tool) in the Web UI for the live
view.

**Implementation tasks.**
- Agent: expose `/metrics` with node-level gauges (CPU%, RAM used/total,
  disk used/total) and per-container status
- Control plane: expose `/metrics` with cluster-level gauges (node
  count, online/offline, workload count by status) aggregated from the
  latest heartbeats
- Web UI: add historical charts (CPU/RAM/disk over the last N hours) to
  the existing node and workload views — still polling the control
  plane's normal JSON endpoints, not `/metrics` directly (`/metrics` is
  for Prometheus; a small retention table in Postgres, or simply
  recomputing from recent heartbeat rows, backs the UI's own charts)
- Optional: this is also a natural point to introduce a long-lived
  gRPC stream between agent and control plane for lower-latency status
  push, as a learning exercise and UX improvement — see
  [`GO_LEARNING_PATH.md`](GO_LEARNING_PATH.md). Not required; polling
  remains correct and simpler.

**Expected outcome.** Pointing an external Prometheus at both a node
agent and the control plane successfully scrapes real metrics; the Web
UI now shows resource trends over time, not just a live snapshot,
without needing Prometheus at all for basic use.

**Do NOT build yet.** No custom time-series storage, no built-in
alerting engine, no log aggregation pipeline (raw `ambudctl logs
<workload>` streaming from containerd is sufficient for now).

**Definition of done.** `/metrics` endpoints exist and are scrapeable;
the Web UI shows real-time cluster and workload status without manual
polling by the operator.

---

## Phase 9 — Authentication & API hardening

**Goal.** Stop assuming "whoever can reach the control plane's API is
trusted" — add real user authentication, distinct from the node
join-token mechanism already in place since Phase 3.

**What we're building.** Username/password login issuing a JWT (or
opaque session token) for the CLI/Web UI, a `users` table, and
middleware enforcing auth on all non-login control-plane endpoints.
Single default admin role — no RBAC yet.

**Why it matters.** Everything before this phase has been correctly
scoped to defer this (per the design principle of "security considered
from the start, advanced features later") — but a cluster you're
actually going to run needs to not be wide open to anyone on the LAN.
This is also the natural point to introduce per-token scoping for future
API clients (e.g., CI systems deploying to Ambud).

**Technologies.** `golang-jwt/jwt`, `golang.org/x/crypto/bcrypt` for
password hashing, HTTP middleware.

**Implementation tasks.**
- `users` table (bcrypt password hash, not plaintext — obviously, but
  stated because it's a real mistake to catch in review)
- `POST /v1/auth/login` → JWT; middleware validating it on protected
  routes
- `ambudctl login`, credential storage in `~/.ambud/config.yaml`
  (file permissions locked down — `0600`)
- Web UI login screen + token storage (careful: document XSS
  implications of `localStorage` vs. an httpOnly cookie, choose
  deliberately rather than by default)
- Distinguish clearly, in code and docs, between **user auth** (this
  phase) and **node join tokens** (Phase 3) — different credentials,
  different trust boundaries, should not be unified just because both
  are "a token"
- Seed a default admin user on first control-plane startup (with a
  generated password printed once, not hardcoded)

**Expected outcome.** An unauthenticated request to any workload/node
endpoint is rejected; `ambudctl login` followed by normal commands works
as before; the Web UI requires login.

**Do NOT build yet.** No RBAC / multiple roles / per-resource
permissions, no SSO/OAuth integration, no audit log (all reasonable
future work, not needed for a single-operator cluster).

**Definition of done.** No control-plane API route (except login and
node registration, which uses its own separate join-token mechanism) is
reachable without valid user auth; this is verified by a test that
asserts a 401 on every protected route without a token.

---

## Phase 10 — Production hardening

**Goal.** Make the cluster something you'd trust running unattended for
weeks, not just something that works in a demo.

**What we're building.** TLS everywhere (control plane API, agent
connections — moving from bearer tokens over plain HTTP to TLS,
optionally mTLS for agent connections), safe control-plane upgrades
(schema migrations that don't lose data, agent version compatibility),
backup guidance for Postgres, and resilience under real failure modes
(control plane restart mid-heartbeat, Postgres briefly unavailable,
agent process crash-and-restart via systemd).

**Why it matters.** This is the difference between a working prototype
and something you can point at as "a real project" in the README's
status line. It's also, honestly, where a lot of the unglamorous but
essential engineering work lives — and skipping it is how homelab
projects stay toys forever.

**Technologies.** TLS (Go's `crypto/tls`, Let's Encrypt/self-signed certs
for a homelab context), systemd `Restart=on-failure`, `pg_dump`-based
backup scripting, migration tooling already in place since Phase 3.

**Implementation tasks.**
- TLS on the control plane API (self-signed CA for LAN use is
  documented as the realistic default — most Ambud clusters won't have
  public DNS/ACME available)
- Agent-to-control-plane auth upgraded from bearer token to mTLS, or at
  minimum bearer-token-over-TLS if mTLS proves too heavy for this stage
  — document whichever tradeoff is chosen and why
  (see [ADR 0001](ADR/0001-initial-architecture.md) style — add ADR 0002
  if the decision is non-trivial)
- systemd units for both binaries with `Restart=on-failure`,
  `WantedBy=multi-user.target`, sane resource limits
- Migration safety: every schema change is additive/backward-compatible
  within a phase-appropriate window; document the upgrade order
  (control plane before agents, or vice versa) and why
- Backup runbook: `pg_dump` cron job documented in `deploy/`, with a
  tested restore procedure (a backup nobody has restored isn't a backup)
- Chaos-ish manual test pass: kill Postgres for 30s during agent
  heartbeats, kill the control plane mid-deploy, kill an agent mid-
  container-start — confirm the system recovers without operator
  intervention in each case, document any that don't
- Structured, leveled logging reviewed end-to-end; sensitive values
  (tokens, passwords) confirmed never logged

**Expected outcome.** A cluster that survives the failure scenarios
above without data loss or manual recovery steps, running over TLS, with
a documented backup/restore procedure someone other than you could
follow.

**Do NOT build yet.** No multi-region, no active-active control plane,
no zero-downtime control-plane upgrades (a brief control-plane restart
during upgrade is an accepted tradeoff — data plane keeps running
regardless, per the architecture's design principles). No compliance/
audit tooling.

**Definition of done.** All chaos-test scenarios in the implementation
tasks pass; TLS is mandatory (not optional) for all network
communication; backup/restore has been executed at least once against a
non-trivial dataset.

---

## A note on UI/UX design polish

The Web UI is required and functional from Phase 3 onward (see each
phase above), but "functional" and "polished" are deliberately kept
separate. Every phase's UI work through the MVP (Phase 5) and beyond is
scoped to plain, working screens — real forms, real data, no dead ends
— not a designed product. A dedicated UI/UX pass (a real design system,
onboarding flow, empty states, visual identity, the things that make a
tool feel like a *product* rather than a working prototype) is treated
as its own deliberate, separate body of work, best tackled once the
underlying screens and flows have stabilized through real use — roughly
once Phase 9 (auth) is in place and the UI's information architecture
has had a chance to prove itself, rather than polishing screens that are
still likely to change shape. This is a value the project holds
("world-class," not just "working"), sequenced deliberately rather than
skipped.

## After Phase 10

Not roadmapped in detail yet, deliberately — revisit only once the above
is real and in use. Candidate future directions, roughly in order of
likely value: a dedicated UI/UX design pass (see above), RBAC/multi-user
teams, an overlay network for true cross-node container networking, a
plugin/extension point for the scheduler, packaged installers (`.deb`/
`.rpm`), a Terraform provider or similar for declarative cluster config.
None of these should be started early — see the "extensible without
prematurely implementing" principle in the project brief.
