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

## Running ambudctl directly against an agent (Phase 2 debug mode)

`ambudctl run`/`ps`/`stop` still talk straight to one `ambud-agent`,
bypassing the control plane entirely — useful for low-level, single-
machine debugging (see docs/ARCHITECTURE.md). From inside the Lima VM
(or any Linux box with containerd running):

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
```

`--agent` defaults to `http://localhost:8080`, matching the agent's
default `--listen`. Image references must be fully qualified
(`docker.io/library/nginx:alpine`, not `nginx:alpine`) — containerd's
client, unlike the `docker` CLI, doesn't implicitly expand short Docker
Hub names.

## Container networking (Phase 6)

Containers get real connectivity — a bridge, an IP, outbound NAT, and
(with `--port`) inbound host-port forwarding — via the CNI bridge +
portmap plugins (see `internal/runtime/network.go` and the ADR's "why
not build our own networking" reasoning). `deploy/lima/ambud-dev.yaml`
provisions the plugin binaries into `/opt/cni/bin` automatically; on
any other Linux box, install them yourself (e.g. from
[containernetworking/plugins releases](https://github.com/containernetworking/plugins/releases),
matching the version pinned in that Lima config). `ambud-agent` writes
its own default CNI config to `/etc/cni/net.d/10-ambud.conflist` on
first startup if nothing is there yet — no separate config step.

`run`/`deploy` take a repeatable `--port hostPort:containerPort[/protocol]`
flag, matching `docker run -p` syntax minus the host-IP prefix (Ambud
binds every mapping to all interfaces):

```sh
./bin/ambudctl --agent http://127.0.0.1:8080 run docker.io/library/nginx:alpine --port 8080:80
curl http://127.0.0.1:8080/   # reaches nginx inside the container
```

## Running Postgres locally

```bash
docker compose -f deploy/compose/postgres.yaml up -d
```

This seeds two databases: `ambud` (for `ambud-controlplane` itself) and
`ambud_test` (for `internal/controlplane/store`'s tests — see that
package's own test files for why they run against real Postgres rather
than a fake). If port 5432 is already taken on your machine, set
`AMBUD_POSTGRES_PORT` before starting it, and update `--db-dsn`/
`AMBUD_TEST_DATABASE_URL` below to match.

`ambud-controlplane` runs its own migrations automatically on startup
(they're embedded in the binary — see `internal/controlplane/store`),
so there's no separate migrate step.

## Running the full stack (Phase 3): control plane + agent + ambudctl

This is the real, tested cluster workflow. Postgres and the control
plane can run on your host (they don't need containerd); the agent
needs the Lima VM (or another Linux box with containerd).

```bash
# host: Postgres, then the control plane
docker compose -f deploy/compose/postgres.yaml up -d
go build -o bin/ambud-controlplane ./cmd/ambud-controlplane
./bin/ambud-controlplane --listen :8081 \
  --db-dsn "postgres://ambud:devpassword@localhost:5432/ambud?sslmode=disable"

# host: generate a join token
./bin/ambudctl --controlplane http://localhost:8081 node generate-join-token
# -> prints a token; copy it

# inside the Lima VM: the agent, pointed at the control plane on the host
# (Lima's VMs can reach the host at its LAN IP; "localhost" from inside
# the VM means the VM itself, not your Mac)
sudo ./bin/ambud-agent --listen 127.0.0.1:8080 \
  --controlplane http://<host-ip>:8081 \
  --join-token <token-from-above> \
  --node-name dev-node-1

# host: see the node, then deploy something to it
./bin/ambudctl --controlplane http://localhost:8081 node list
./bin/ambudctl --controlplane http://localhost:8081 deploy docker.io/library/nginx:alpine
./bin/ambudctl --controlplane http://localhost:8081 workloads list
```

The deploy doesn't start the container immediately — the assigned
node's agent picks it up on its next heartbeat (`--heartbeat-interval`,
default 5s). Re-run `workloads list` a few seconds later to see it
`running`.

The agent persists its control-plane credential to
`--credentials-path` (default `/var/lib/ambud/agent/credentials.json`)
after its first successful registration — delete that file to force
re-registration (you'll need a fresh join token; each one is single-use).

For anything past a quick manual test, install the systemd units from
`deploy/systemd/` instead of running via `go build && ./bin/...` by
hand — this also validates the actual production startup path
(restart-on-failure, logging to journald) rather than only the dev
shortcut.

## Running tests

```bash
make test          # go test ./... — unit tests, no external deps required
make test-race      # go test -race ./... — see GO_LEARNING_PATH.md #7
make lint           # golangci-lint run
```

Most tests don't need a live Postgres or containerd — that's the point
of the interface-driven design in
[`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md) and
[`GO_LEARNING_PATH.md`](GO_LEARNING_PATH.md) #9. `internal/controlplane/store`
is the one deliberate exception: it tests against a real Postgres, not
a fake, because the whole point of that package is the SQL and the
constraints it depends on (unique names, foreign keys) — a fake
couldn't validate those honestly. `internal/agent` and `internal/cpclient`
add a further layer on top, driving their real HTTP clients against the
real control-plane API server backed by that same real Postgres.

These tests don't need a separate `make test-integration` target or
build tag — they run as part of the normal `go test ./...` / `make
test`, but `t.Skip` themselves (not fail) if no Postgres is reachable
at `AMBUD_TEST_DATABASE_URL` (default `postgres://ambud:devpassword@localhost:15432/ambud_test?sslmode=disable`).
Start one with `docker compose -f deploy/compose/postgres.yaml up -d`
(the compose file's default port is 5432 — either set
`AMBUD_POSTGRES_PORT=15432` when starting it, or point
`AMBUD_TEST_DATABASE_URL` at whatever port you used) or the throwaway
`docker run` a store/agent/cpclient test failure message will show you.
CI provides Postgres via a service container in `.github/workflows/ci.yml`
so these run on every push regardless of what a contributor has set up
locally.

## Local multi-node development

**The tested setup: two separate Lima VMs, control plane on the host.**
`deploy/lima/ambud-dev.yaml` can be started under different instance
names to get two independent Linux boxes — separate kernels, separate
containerd, separate network namespaces — not just two processes on one
machine:

```sh
limactl start --name=ambud-dev deploy/lima/ambud-dev.yaml    # node 1
limactl start --name=ambud-dev-2 deploy/lima/ambud-dev.yaml  # node 2
```

Each Lima VM gets its own isolated NAT network — the two VMs cannot
reach each other directly, and (surprisingly, until you check) may even
get the *same* private IP, since each is a separate /24. What every Lima
VM *can* reach is the host, via the special hostname `host.lima.internal`
(confirm with `limactl shell ambud-dev -- cat /etc/hosts`). Since
`ambud-controlplane` doesn't need containerd, the natural topology is:
Postgres + the control plane run on the host (`--listen 0.0.0.0:8081`
so the VMs can actually reach it, not `127.0.0.1`), and each VM runs
only `ambud-agent --controlplane http://host.lima.internal:8081 ...`.
`ambudctl` also runs on the host, talking to `127.0.0.1:8081`.

This is genuinely two independent machines for everything that matters
(agent code, containerd, the network hop to the control plane over real
HTTP) — not a simulation. It's just not two different pieces of
physical hardware. If you want that extra layer of confidence — real
latency, a real router, a genuinely separate kernel's resource limits —
your repurposed old PC (or any spare machine) as node 2, reachable over
your actual LAN, is the natural next step; nothing about the setup above
changes except which hostname/IP `--controlplane` and `host.lima.internal`
resolve to.

One gotcha worth knowing in advance: if you reset the control plane's
database (e.g. `DROP DATABASE ambud`) while an agent still has a saved
credential file (`--credentials-path`, default
`/var/lib/ambud/agent/credentials.json`) from before the reset, it will
try to heartbeat with a credential the fresh database has never heard
of and fail with "invalid credential" in a loop. Delete that file
before restarting the agent in that situation — it'll register fresh
with a new join token instead of assuming it's already known.

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
