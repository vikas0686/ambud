# Development Setup

Ambud targets Linux in production (see
[ADR 0001](ADR/0001-initial-architecture.md) for why), but you will
likely be *editing* code on macOS or Windows. This document explains how
to bridge that gap without it slowing you down, plus how to run every
component locally.

## The Windows/macOS-dev vs. Linux-target problem

containerd, cgroups, and namespaces are Linux concepts — there's no
meaningful way to run the real container runtime path on macOS, and
Windows containerd is a different, unrelated thing from what Ambud
targets. So the split is:

- **Code that doesn't touch containerd** (CLI argument parsing, HTTP
  handlers with a fake runtime, the scheduler, the store layer against a
  containerized Postgres, most unit tests) — this compiles and runs
  fine on macOS or Windows directly. Do this work with your normal local
  Go toolchain, no VM needed.
- **Code that does touch containerd** (`internal/runtime`, and any
  agent integration test that exercises a real container) — this
  **must** run on Linux. You need a Linux environment for this, even if
  it's just for the final "does this actually work" check.

Given you mentioned having an old Windows PC available: the fastest path
depends on which machine is your primary editor.

### If your primary dev machine is macOS (matches this environment)

1. Write and unit-test code locally on macOS with a normal Go install —
   this covers the majority of the codebase (see split above).
2. For anything that needs real containerd (Phase 1 onward), use the
   checked-in Lima VM config, [`deploy/lima/ambud-dev.yaml`](../deploy/lima/ambud-dev.yaml):

   ```sh
   limactl start deploy/lima/ambud-dev.yaml   # first run: downloads a base
                                                # image (~700MB) and provisions
                                                # Go + gcc — several minutes
   limactl shell ambud-dev
   ```

   This is the actual, tested setup Phase 1 was validated against — not
   a generic "use a VM" pointer. It gives you **system-wide (rootful)
   containerd**, matching how Ambud runs in production (Lima's own
   default is rootless, which uses a different socket and doesn't
   reflect the target deployment), a provisioned Go toolchain + `gcc`
   (needed for `go test -race`), and the repo mounted read-write at the
   *same absolute path* as on the host — so `cd` to your normal repo
   path inside the VM and everything (`go build`, `go test`, `sudo
   ./bin/ambudctl run ...` against the VM's containerd) just works.
   `ambudctl` needs root to reach containerd's socket at this stage
   (`/run/containerd/containerd.sock` is `root:root`) — run it with
   `sudo` inside the VM, or revisit this once Phase 2's agent takes over
   talking to containerd as a dedicated service user.

   Other Linux-VM options (OrbStack, Multipass, plain UTM) work too if
   you'd rather not use Lima — you'll just need to install containerd,
   Go, and `gcc` yourself, and enable rootful (not rootless) containerd.
3. **Best option once you're past Phase 1:** repurpose the old Windows
   PC as your actual Linux test node. Install a lightweight server Linux
   distro on it (Debian or Ubuntu Server — avoid anything exotic, this
   should be the same distro family you'd realistically run in
   production), and SSH into it from your Mac to run the agent and
   containerd for real. This has a nice property: it's not just a dev
   convenience, it's literally the target hardware the project is
   about — "an old computer becomes a private cloud node" — so testing
   against it from Phase 4 onward (multi-node) doubles as validating the
   project's own premise.

### If your primary dev machine is the Windows PC

- Use **WSL2** with a real Linux distro (Ubuntu is the well-trodden
  path). WSL2 runs a real Linux kernel, so containerd/runc work
  natively inside it — no VM-in-VM weirdness. Install Go, containerd,
  and do all development inside the WSL2 distro, not in native Windows
  Go.
- In this case, keep the "old PC" framing in mind: if this machine is
  meant to *become* a node rather than stay your editor, consider
  flipping the setup — dual-boot or wipe it to Linux, and edit code from
  a different machine (or over SSH/VS Code Remote) instead. Decide this
  once, early, rather than developing against WSL2 long-term and
  discovering later you want the box as a clean Linux node.

Either way, by **Phase 4** (multi-node), you need at least two real (or
virtual) Linux machines regardless of dev-machine choice — that's not
optional at that point, it's the thing being tested.

## Required tools

| Tool | Version | Used for |
|---|---|---|
| Go | 1.26.3+ | All backend code — see `go.mod`'s `go` directive for the current floor (pulled up from 1.23 by containerd's own go.mod; check `go.mod` rather than this table if they've since diverged) |
| Node.js | 20 LTS+ | Web UI |
| npm or pnpm | current | Web UI package management (pick one, be consistent) |
| PostgreSQL | 15+ | Control plane state store |
| containerd | 1.7+ | Container runtime (Linux only — see above) |
| `runc` | (ships with containerd) | OCI runtime containerd drives |
| Docker or Podman | current | *Only* for running Postgres locally in a container — not used for the actual Ambud workload path |
| `golangci-lint` | current | Linting, matches CI |
| `golang-migrate` or `goose` | current | Postgres migrations (pick one in Phase 3, stay consistent) |
| `git` | current | — |

Go version policy: track the current Go release minus one (i.e., stay
within the last two Go releases) rather than pinning indefinitely —
update `go.mod`'s `go` directive deliberately when bumping, not
accidentally via toolchain auto-update.

## Running ambudctl + ambud-agent today (Phase 2)

As of Phase 2, `ambudctl` talks to `ambud-agent` over HTTP — it no
longer touches containerd directly (that was Phase 1's scaffolding).
There's still no control plane (Phase 3; the "Running the control
plane" / "Running an agent" sections below describe that *future*
workflow with join tokens, `--controlplane`, etc.). From inside the
Lima VM (or any Linux box with containerd running):

```sh
cd /path/to/ambud   # same path as on the host, if using the Lima VM above
go build -o bin/ambud-agent ./cmd/ambud-agent
go build -o bin/ambudctl ./cmd/ambudctl

# terminal 1: the agent (needs root — containerd's socket is root:root)
sudo ./bin/ambud-agent --listen 127.0.0.1:8080

# terminal 2: ambudctl talks HTTP to it, no sudo needed
./bin/ambudctl --agent http://127.0.0.1:8080 run docker.io/library/nginx:alpine
./bin/ambudctl --agent http://127.0.0.1:8080 ps
./bin/ambudctl --agent http://127.0.0.1:8080 stop nginx

# resource facts (also used by the future scheduler — see docs/ROADMAP.md Phase 5)
curl -s http://127.0.0.1:8080/v1/resources
```

`--agent` defaults to `http://localhost:8080`, matching the agent's
default `--listen`. `--socket` (containerd's socket path) moved from
`ambudctl` to `ambud-agent`, since the agent is now the only thing that
talks to containerd — `ambudctl`'s only network target is the agent.

Image references must be fully qualified (`docker.io/library/nginx:alpine`,
not `nginx:alpine`) — containerd's client, unlike the `docker` CLI,
doesn't implicitly expand short Docker Hub names.

## Running Postgres locally

Don't hand-install Postgres on your dev machine — run it in a container,
even on the Linux node, for easy reset/teardown during development:

```bash
docker run --name ambud-postgres \
  -e POSTGRES_USER=ambud -e POSTGRES_PASSWORD=devpassword -e POSTGRES_DB=ambud \
  -p 5432:5432 -d postgres:16
```

A `deploy/compose/postgres.yaml` (Phase 3) should codify this so it's
not a memorized command. Migrations run against this instance via
whichever tool was chosen in Phase 3 (`golang-migrate` or `goose`).

## Running the control plane

```bash
# from repo root, once Phase 3 exists
go run ./cmd/ambud-controlplane \
  --db-dsn "postgres://ambud:devpassword@localhost:5432/ambud?sslmode=disable" \
  --listen :8081
```

## Running an agent (once Phase 3 exists)

This describes the *future* control-plane-connected agent workflow —
for how to actually run `ambud-agent` today, see "Running ambudctl +
ambud-agent today (Phase 2)" above; today's agent takes no
`--controlplane` or `--join-token` flags. The agent needs a real Linux
environment with containerd running (see above). From that environment:

```bash
# one-time: generate a join token from the control plane
ambudctl node generate-join-token --controlplane http://<control-plane-host>:8081

# start the agent with it
go run ./cmd/ambud-agent \
  --controlplane http://<control-plane-host>:8081 \
  --join-token <token-from-above> \
  --listen :8080
```

For anything past a quick manual test, install the systemd unit from
`deploy/systemd/ambud-agent.service` (added in Phase 2) instead of
running via `go run` — this also validates the actual production startup
path (restart-on-failure, logging to journald) rather than only the dev
shortcut.

## Running tests

```bash
make test          # go test ./... — unit tests, no external deps required
make test-race      # go test -race ./... — see GO_LEARNING_PATH.md #7
make lint           # golangci-lint run
```

Unit tests must not require a live Postgres or containerd — that's the
point of the interface-driven design in
[`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md) and
[`GO_LEARNING_PATH.md`](GO_LEARNING_PATH.md) #9. Anything that
genuinely needs a live dependency belongs in a separate, explicitly-
named integration test target (e.g., `make test-integration`, added
once Phase 3's store layer exists), which CI can choose to run
differently (or not at all on every push) from the fast unit suite.

## Local multi-node development

Once you're past Phase 4, local multi-node testing has two reasonable
setups:

1. **Two Linux VMs on one machine** (fine through Phase 5–6 for basic
   functional testing) — e.g., two Lima/Multipass instances, both
   pointed at a control plane running on your host or in a third VM.
   Cheap to spin up and tear down, good for fast iteration.
2. **Real separate physical machines** (needed eventually, and
   recommended from Phase 4 onward at least once per phase) — this is
   the only way to catch real-network issues (actual latency, real NAT/
   firewall behavior, genuine hardware resource limits) that two VMs
   sharing one host's kernel and network stack will never surface. Your
   repurposed old Windows-PC-turned-Linux-box is the natural node #2
   here, with your primary dev machine (or a third box) running the
   control plane.

Use VMs for day-to-day iteration, and validate against real separate
hardware before considering a phase actually "done" — the roadmap's
Definition of Done for Phase 4 explicitly calls for two real physical
machines for this reason.

## Web UI development

```bash
cd web
npm install
npm run dev   # dev server with hot reload, proxying API calls to the
              # control plane (configure the proxy target in web/vite.config.ts
              # or equivalent once Phase 0's web scaffold exists)
```

The Web UI only needs the control plane's REST API reachable — it has no
containerd or Postgres dependency of its own, so it can be developed
from macOS/Windows natively with no VM involved, pointed at a control
plane running wherever (local VM, remote Linux box).

Since the Web UI is a required, first-class client from
[Phase 3](ROADMAP.md#phase-3--control-plane) onward (see
[`ARCHITECTURE.md`](ARCHITECTURE.md) and [`MVP.md`](MVP.md)), `npm run
dev` alongside `go run ./cmd/ambud-controlplane` should be treated as
part of the normal everyday dev loop from that point forward, not
something reached for occasionally — any change to the control-plane API
should be checked against the UI in the same sitting, the same way you'd
check `ambudctl`.

## Editor setup

Any editor with a Go language server (`gopls`) works fine — VS Code with
the official Go extension, or GoLand, are the least-friction choices if
you don't already have a strong preference. Enable `goimports`-on-save
so import formatting matches what CI's `gofmt` check expects, avoiding
noisy formatting-only diffs in PRs.
