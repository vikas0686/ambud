# ADR 0001: Initial Architecture

- **Status:** Accepted
- **Date:** 2026-08-25

## Context

Ambud needs an initial set of architectural decisions before any code is
written: language choices, storage, workload abstraction, and the
overall shape of the system (control-plane/agent vs. alternatives). This
ADR records those decisions and, per the project's own principle of
explaining tradeoffs rather than presenting decisions as absolute
truths, the alternatives that were considered and why they were set
aside — for *this* project, at *this* stage, not as a universal
judgment.

These decisions are also constrained by two facts about the project
itself: it's built by a solo developer who does not yet know Go and
wants to learn it *through* this project (see
[`GO_LEARNING_PATH.md`](../GO_LEARNING_PATH.md)), and it targets
genuinely modest hardware (old PCs), not a datacenter fleet.

## Decisions

### Why Go

**Decision:** Go for the control plane and node agent.

Go is the dominant language of the infrastructure/orchestration space
for concrete reasons that apply directly here: `containerd`, `runc`,
Docker, Kubernetes, and most of the CNCF ecosystem are written in Go, so
Ambud's core dependency (containerd's client library) is a first-class,
idiomatic Go API rather than a bindings layer. Go also compiles to
static, single-file binaries with no runtime to install on a target
machine — a meaningful property for "install an agent on an old PC,"
where you don't want to also manage a language runtime/VM on that
machine. Goroutines make the agent's inherently concurrent workload
(serve HTTP, poll containerd, heartbeat, all at once) natural to express.

**Alternatives considered:**
- **Rust** — comparable deployment story (static binaries), arguably
  stronger safety guarantees, and a real presence in this space
  (`youki`, parts of the container ecosystem). Set aside primarily
  because of the learning-curve stack: the user wants to learn one new
  thing (Go) while building a real system, not two (Rust's ownership
  model *and* systems programming) — and Go's ecosystem fit
  (containerd's own client library) is more direct.
- **Python** — fastest to prototype in, and the user's likely comfort
  language as a backend developer, but a poor fit for a long-running
  daemon distributed as a single binary to arbitrary machines (dependency/
  interpreter management on the target node), and it's not the language
  the container ecosystem's libraries are written in.
- **TypeScript/Node everywhere** (including backend) — would unify the
  language across the whole stack, but Node is a weaker fit for
  low-level system/process/resource work and has no equivalent to
  containerd's native Go client.

### Why React/TypeScript for the Web UI

**Decision:** TypeScript + React for the Web UI, as a fully separate
project from the Go backend (see
[`PROJECT_STRUCTURE.md`](../PROJECT_STRUCTURE.md)).

React + TypeScript is a mainstream, well-documented choice with the
largest available talent pool and component ecosystem for a dashboard-
style UI (tables, live-updating cards, forms) — this is not a place
where Ambud benefits from a novel choice. TypeScript's static typing
also pairs well with generating client types from the REST/OpenAPI
contract (`api/openapi.yaml`), catching drift between backend and
frontend at compile time.

**Alternatives considered:**
- **Server-rendered Go templates (`html/template`)** — would avoid a
  second language/toolchain entirely, and is genuinely tempting for
  minimalism. Set aside because the Web UI is the primary, required
  interface to Ambud (see [`ARCHITECTURE.md`](../ARCHITECTURE.md) and
  [`MVP.md`](../MVP.md)), not a minor add-on — it needs live-updating
  state (node status, resource graphs, container status) that's a much
  better fit for a client-side framework's state management than
  template re-renders, and it's worth the second toolchain given how
  central it is to the product.
- **Vue/Svelte** — both reasonable, smaller/simpler than React in some
  respects. Set aside in favor of React's larger ecosystem and the fact
  that it's the more likely skill a future contributor already has.

### Why PostgreSQL

**Decision:** PostgreSQL as the control plane's only datastore.

Postgres is mature, well understood, has excellent Go driver support
(`pgx`), handles the relational shape of Ambud's data well (nodes,
workloads, and their relationships), and — critically — is a single
well-known operational surface: one process to run, back up
(`pg_dump`), and reason about. It comfortably handles the request volume
a homelab-scale control plane will ever see; this is nowhere near a
scale where Postgres is the bottleneck.

**Alternatives considered:**
- **SQLite** — genuinely attractive for a homelab-scale single-process
  control plane: zero extra process to run, one file to back up. Set
  aside mainly for headroom rather than a strong technical objection:
  Postgres gives more comfortable concurrent-write behavior as the
  control plane fields simultaneous heartbeats from many nodes (Phase 4+),
  and the operational cost of one more container (`docker run postgres`)
  is low. This is a close call, not a strong rejection — a future
  contributor proposing SQLite for simpler deployments would be raising
  a fair point, and it's worth revisiting if Ambud's deployment story
  ever prioritizes "single binary, zero external dependencies" over the
  current design.
- **etcd or another distributed KV store** — the reflexive choice if
  you're influenced by Kubernetes' architecture. Explicitly rejected —
  see "why not start with Kubernetes" and "why not a distributed
  database" below.
- **MySQL** — comparable to Postgres for this use case; no strong reason
  to prefer it over Postgres, and Postgres's ecosystem reputation in the
  Go community tipped the choice.

### Why containers as the workload abstraction

**Decision:** OCI containers, run via containerd, are Ambud's only
workload unit.

Containers are the right level of abstraction for "run arbitrary
software across a fleet of machines" without inventing a new packaging
format, and the ecosystem of existing container images means Ambud is
immediately useful for real software (databases, web servers, anything
on Docker Hub) rather than requiring bespoke packaging.

**Alternatives considered:**
- **VMs** — stronger isolation, but far heavier for old/modest hardware
  (the exact target class of machine this project cares about), and
  would require Ambud to also solve VM image management/networking,
  a substantially bigger undertaking with worse fit for the target
  hardware.
- **Bare processes / systemd units as the workload unit** — simpler in
  one sense (no runtime dependency at all), but loses isolation,
  reproducible environments, and the entire existing container image
  ecosystem. Would make Ambud meaningfully less useful for "deploy
  arbitrary software" out of the gate.

### Why not build a container runtime

**Decision:** Drive containerd's existing client library directly;
never implement container creation, namespaces, or cgroup management
ourselves.

This is close to a non-decision in that building a container runtime
from scratch would be a multi-year undertaking orthogonal to Ambud's
actual goal (a control plane and fleet management layer), duplicating
work that containerd already does correctly and is already what Docker
and Kubernetes rely on in production. There is no version of "learning
Go by building Ambud" that benefits from also reimplementing `runc`.

**Alternative considered:** Directly shelling out to the `docker` CLI
instead of containerd's client library. Rejected because it adds an
unnecessary process boundary and parsing layer (shell out, parse text/
JSON output) where a native Go client (containerd's) is directly
available and more robust; containerd is also the lower-level component
Docker itself is built on, so talking to it directly avoids depending on
the Docker daemon's own extra layer.

### Why not build our own networking

**Decision:** Container networking (Phase 6) is the CNI bridge +
portmap plugins (`github.com/containerd/go-cni`), the same mechanism
nerdctl and Kubernetes use — not a hand-rolled iptables/nftables layer,
and not a custom userspace packet forwarder.

Driving containerd directly (rather than through the CRI shim — see
above) means Ambud gets no networking for free: a container created via
the native client starts in an isolated network namespace with only
loopback, full stop. Building the equivalent of what CNI's bridge
plugin does — a Linux bridge, veth pairs, IPAM, NAT, host-port DNAT —
by hand would mean reimplementing a well-understood, already-correct
piece of infrastructure for no benefit; the actual hard problem in this
space (cross-node overlay networking) is explicitly out of scope for
Phase 6 in the first place (see `ROADMAP.md`). CNI's plugin model also
means the only Ambud-specific code is a thin Go wrapper (`go-cni`) and
a bundled default config (`internal/runtime/network.go`) — the
namespace/bridge/NAT mechanics themselves are upstream, audited code.

Each container gets its own named, persistent network namespace,
created with `ip netns add` and referenced by a stable path — not a
namespace derived from the container's process PID
(`/proc/<pid>/ns/net`). A PID-derived path stops resolving the instant
the process exits, which is exactly when crash-cleanup needs it most;
a named namespace tears down the same way whether the container
stopped cleanly, crashed, or never started.

**Alternative considered:** nerdctl's own approach, which sets up CNI
networking via an OCI runtime prestart hook (an internal `nerdctl
internal oci-hook` self-exec invoked by the runtime at precise
container-creation lifecycle timing). Rejected as more machinery than
Ambud's much narrower single-default-network use case needs — nerdctl's
hook-based design exists to support multiple networks and plugins
chosen per-container at `nerdctl run` time, a flexibility Phase 6 has no
requirement for. Setting up networking directly in `Run()`, before the
task starts, is simpler and gives the same correctness guarantee (no
window where the container is running but unreachable).

### Why not start with Kubernetes

**Decision:** Ambud does not use, embed, or expose a Kubernetes-
compatible API, and does not build on top of `k3s`/`k0s`/minikube-style
lightweight Kubernetes distributions.

Kubernetes is designed for a scale and failure-mode space (many nodes,
many teams, high churn workloads, cloud-provider integrations) that
doesn't match "two or three old PCs in a closet." Its operational
surface — etcd, kubelet, CNI plugins, kube-proxy, a YAML-based API
model with dozens of resource kinds — is substantial complexity that
would dominate the project even at k3s's "lightweight" end, and would
turn Ambud into "assemble Kubernetes" rather than "build and understand
a control plane from first principles," undermining the explicit
learning goal. It would also mean most of the interesting engineering
(scheduling, node registration, reconciliation) happens *inside*
Kubernetes' machinery rather than in Ambud's own code — the opposite of
what makes this project worth doing.

**Alternative considered:** Build Ambud as a thin UI/CLI layer over k3s.
This would ship a working multi-node platform faster, but converts
Ambud into an integration project rather than an infrastructure project,
forecloses the learning goal, and inherits Kubernetes' operational
weight for homelab-scale hardware that doesn't need it. If Ambud ever
outgrows its own scheduler/control-plane design at real scale,
revisiting this is reasonable then — but not as a starting point.

### Why a control-plane/agent architecture

**Decision:** A central control plane holding desired state, and a node
agent per machine that pulls that state and reconciles it locally (full
detail in [`ARCHITECTURE.md`](../ARCHITECTURE.md)).

This is the proven shape for exactly this problem (Kubernetes' API
server/kubelet split, Nomad's server/client split, and many others share
it) because it cleanly separates *decisions* (control plane) from
*execution* (data plane/agent), which is what allows the control plane
to be temporarily unavailable without stopping already-running
workloads — a property directly required for a system running on
consumer-grade, occasionally-rebooted hardware.

**Alternatives considered:**
- **Fully peer-to-peer / no central control plane** (gossip-based
  cluster membership, decisions made collectively) — removes the single
  point of coordination but at a large complexity cost (distributed
  consensus for placement decisions) for no real benefit at this scale;
  this is solving a problem Ambud doesn't have (thousands of nodes,
  adversarial/untrusted membership).
- **Single monolith with SSH-based remote execution** (control plane
  SSHes into nodes to run commands directly, no persistent agent) —
  simpler to start, but pushes the control plane into the request path
  of every operation (violating the control/data-plane separation
  above), requires the control plane to always be able to *reach* every
  node (breaking the "agents pull, never pushed to" NAT-friendliness
  described in [`ARCHITECTURE.md`](../ARCHITECTURE.md)), and gives up
  the local reconcile-loop resilience property entirely.

### Why not build a distributed database

**Decision:** One Postgres instance is the system of record. No Raft/
etcd-style consensus layer is built or embedded.

Distributed consensus is one of the hardest problems in this space, and
building one specifically to store a few hundred rows of node/workload
state for a homelab cluster would be enormous, unjustified complexity —
literally reimplementing etcd, badly, for no benefit at this scale. A
single Postgres instance is a single point of failure for the *control
plane* (not for running workloads, per the control/data-plane
distinction), which is an acceptable, well-understood tradeoff at this
scale, mitigated by ordinary backup practice (Phase 10) rather than by
building distributed consensus.

**Alternative considered:** Postgres streaming replication with a
warm/hot standby, from the start. Reasonable infrastructure practice in
general, but premature here — it adds real operational complexity
(failover logic, split-brain avoidance) before there's a demonstrated
need, and directly conflicts with the "grow into complexity" principle.
Worth revisiting once Ambud is actually being run unattended for
extended periods (Phase 10 territory), not before.

### Why Linux is the target platform

**Decision:** Linux is the only supported platform for the node agent
and control plane in production. Development may happen on macOS/
Windows (see [`DEVELOPMENT.md`](../DEVELOPMENT.md)), but that's a dev-
convenience concern, not a target-platform one.

containerd, cgroups, and Linux namespaces — the actual isolation
mechanisms underneath any container runtime — are Linux kernel features
with no equivalent on macOS or Windows (Windows containers are a
different, non-overlapping technology). Since Ambud explicitly commits
to containerd rather than building its own runtime, it inherits
containerd's platform requirement. This also matches the project's own
premise: the target hardware ("old computers") most usefully and
cheaply runs Linux.

**Alternative considered:** Cross-platform support via a Windows-
specific or macOS-specific runtime backend. Rejected as scope the
project doesn't need — the entire premise is repurposing old hardware
into Linux-based cluster nodes; supporting Windows/macOS as *targets*
would roughly double the runtime-integration surface for a use case
outside Ambud's actual goals.

## Consequences

- The project has a hard dependency on Linux for anything beyond
  argument-parsing/business-logic-only code, which shapes
  [`DEVELOPMENT.md`](../DEVELOPMENT.md)'s VM/WSL2 guidance.
- A single Postgres instance and a single control-plane process are
  both, currently, single points of failure. This is accepted and
  documented rather than hidden, and is scoped to be addressed (to the
  extent it ever is) no earlier than [Phase 10](../ROADMAP.md#phase-10--production-hardening).
- Not building on Kubernetes means Ambud will, for a long time, have
  fewer features than a k3s-based alternative. This is accepted as the
  cost of the project's actual goals (learning Go by building real
  infrastructure primitives, staying usably simple at homelab scale) —
  not a gap to apologize for.
- Every exclusion in this ADR is revisitable. None of these are claimed
  as permanent truths — they're the right calls for a solo developer,
  learning Go, targeting a few old machines, today. A future ADR should
  supersede any of these the moment a real, demonstrated need appears,
  not before.

## Related documents

- [`ARCHITECTURE.md`](../ARCHITECTURE.md) — the resulting component
  design
- [`ROADMAP.md`](../ROADMAP.md) — how this architecture gets built
  incrementally
- [`PROJECT_STRUCTURE.md`](../PROJECT_STRUCTURE.md) — how this
  architecture maps to the repo layout
