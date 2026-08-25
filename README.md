# Ambud

[![CI](https://github.com/vikas0686/ambud/actions/workflows/ci.yml/badge.svg)](https://github.com/vikas0686/ambud/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/vikas0686/ambud.svg)](https://pkg.go.dev/github.com/vikas0686/ambud)

**Turn the old computers in your closet into a private cloud.**

Ambud is a lightweight, self-hosted infrastructure platform. Point it at a
handful of machines — an old gaming PC, a spare laptop, a couple of
Raspberry Pis — and get back the boring, useful parts of AWS: a control
plane that knows what's running where, a way to deploy containers to any
machine in the fleet, and a dashboard to see it all. No Kubernetes cluster
to babysit, no cloud bill.

> **Status: Phase 2 — node agent.** `ambud-agent` is a long-running
> daemon exposing a local HTTP API (container lifecycle + resource
> facts); `ambudctl` is now a pure HTTP client of it — no control plane,
> no real UI yet. See [`docs/ROADMAP.md`](docs/ROADMAP.md) for what's
> being built and in what order, and [`docs/MVP.md`](docs/MVP.md) for
> the first real milestone.

## Why does this exist?

Most homelab / self-hosting tooling falls into two camps: single-machine
tools (plain Docker, Portainer) that don't know about your other machines,
or full Kubernetes, which is powerful but heavy — built for fleets of
thousands, not three old PCs under a desk. There isn't much in between.

Ambud is that middle ground: a real control-plane/agent architecture,
built on mature, boring technology (containerd, PostgreSQL, plain REST),
that scales down to "I have two machines" without dragging in etcd,
kubelet, CNI plugins, and a YAML dialect.

It's also a deliberately-chosen vehicle for learning Go by building
something real instead of following a tutorial — see
[`docs/GO_LEARNING_PATH.md`](docs/GO_LEARNING_PATH.md).

## Core vision

- **Controlled entirely through a web dashboard.** Register a node,
  deploy a container, start/stop/restart it, watch its resource usage —
  all by clicking, not by memorizing CLI flags. The CLI (`ambudctl`)
  exists as a first-class peer for scripting and automation, but every
  capability is required to work identically from the UI — see
  [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)'s design principles.
- **Multiple machines, one control plane.** Add a node, it shows up.
  Remove one, workloads elsewhere are unaffected.
- **Containers as the workload unit.** No custom runtime — Ambud drives
  [containerd](https://containerd.io), the same engine Docker and
  Kubernetes use underneath.
- **Boring, provable infrastructure.** PostgreSQL for state, REST (and
  later gRPC) for APIs, systemd for process supervision. Nothing here
  should surprise an experienced backend engineer.
- **Grow into complexity, don't start with it.** No Kubernetes-level
  scheduling, no custom overlay networks, no distributed storage engine —
  until (if ever) the project actually needs them. See the
  [ADR](docs/ADR/0001-initial-architecture.md) for why.

## Architecture, at a glance

```mermaid
flowchart LR
    subgraph Control Plane
        API[API Server] --> SCHED[Scheduler]
        API <--> DB[(PostgreSQL)]
        SCHED <--> DB
    end
    CLI[ambudctl CLI] --> API
    WEB[Web Dashboard] --> API

    subgraph "Node 1"
        AGENT1[Node Agent] --> CTR1[containerd]
    end
    subgraph "Node 2"
        AGENT2[Node Agent] --> CTR2[containerd]
    end

    AGENT1 -- registers / heartbeats / pulls work --> API
    AGENT2 -- registers / heartbeats / pulls work --> API
```

The control plane holds desired state and decides *what should run
where*. Node agents, running on every machine you add to the fleet, pull
that desired state and make it real by driving containerd locally. Full
breakdown, including the control-plane/data-plane split, in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Roadmap

Development is broken into 11 phases, from "hello world CLI" to
"production-hardened multi-node cluster." Each phase ships something
runnable. Full detail in [`docs/ROADMAP.md`](docs/ROADMAP.md); the short
version:

| Phase | Milestone | Status |
|---|---|---|
| 0 | Repo, tooling, CI scaffolding | ✅ done |
| 1 | Single-node prototype (CLI drives containerd directly) | ✅ done |
| 2 | Node agent (long-running service, local API) | ✅ done |
| 3 | Control plane + PostgreSQL (still one node) | ⬜ not started |
| 4 | Second machine joins the cluster | ⬜ not started |
| 5 | Container scheduling across nodes — **MVP** | ⬜ not started |
| 6 | Networking (port exposure, service discovery) | ⬜ not started |
| 7 | Storage (local persistent volumes) | ⬜ not started |
| 8 | Monitoring (resource metrics, Prometheus exposition) | ⬜ not started |
| 9 | Authentication & authorization | ⬜ not started |
| 10 | Production hardening (TLS, upgrades, backups) | ⬜ not started |

See [`docs/MVP.md`](docs/MVP.md) for exactly what the Phase 5 MVP does
and does not include.

## Quick start

`ambud-agent` drives containerd; `ambudctl` talks to the agent over
HTTP — no control plane yet (Phase 3), so this is still one machine at
a time. Needs Linux; see [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md)
for a one-command Lima VM if you're on macOS/Windows.

```sh
git clone https://github.com/vikas0686/ambud.git
cd ambud
make build

# terminal 1
sudo ./bin/ambud-agent --listen 127.0.0.1:8080

# terminal 2
./bin/ambudctl run docker.io/library/nginx:alpine
./bin/ambudctl ps
./bin/ambudctl stop nginx
```

This section grows further once Phase 3 adds the control plane —
deploying to a *fleet*, not just one machine's agent. For a full local
dev environment (Postgres, containerd, the web dashboard), see
[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md).

## Documentation

- [`docs/ROADMAP.md`](docs/ROADMAP.md) — phased build plan
- [`docs/MVP.md`](docs/MVP.md) — the smallest useful version of Ambud
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — component design and data flow
- [`docs/PROJECT_STRUCTURE.md`](docs/PROJECT_STRUCTURE.md) — repo layout and reasoning
- [`docs/GO_LEARNING_PATH.md`](docs/GO_LEARNING_PATH.md) — learning Go through Ambud
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — local environment setup
- [`docs/ADR/0001-initial-architecture.md`](docs/ADR/0001-initial-architecture.md) — key decisions and alternatives considered

## Contributing

Not open for external contribution yet — the project is still
establishing its own foundations (see Phase 0–1 in
[`docs/ROADMAP.md`](docs/ROADMAP.md)). A `CONTRIBUTING.md` with setup
instructions, coding conventions, and PR expectations will be added once
there's working code to contribute to.

In the meantime, `make ci` runs the same checks CI runs (Go build, vet,
`golangci-lint`, race-enabled tests, `gofmt`/tidy verification, and the
web dashboard's lint/format/build) — every commit in this repo is
expected to pass it.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
