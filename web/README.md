# Ambud Web Dashboard

The primary way an operator uses Ambud — see
[`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) for why this isn't
a secondary/optional interface, and
[`../docs/ROADMAP.md`](../docs/ROADMAP.md) for which phase adds which
screen.

React + TypeScript + Vite. No state management library, UI kit, or
router yet — deliberately: as of Phase 3 there's exactly one screen
(node list + deploy form + workload list, all on `App.tsx`), so a
router has nothing to route between yet, and three plain `useState`
polling components haven't earned a state library. Add these once a
second screen actually needs them, not before.

## Talking to the control plane

Requests go straight to `ambud-controlplane`'s REST API — no proxy, no
BFF layer. `VITE_CONTROLPLANE_URL` overrides the default
(`http://localhost:8081`, matching the control plane's own `--listen`
default) if yours runs elsewhere; see `src/api/client.ts`.

TypeScript types for every request/response shape are generated from
[`../api/openapi.yaml`](../api/openapi.yaml) — the same contract file
`internal/apitypes` (Go) is meant to match — via `npm run generate:api`
(`openapi-typescript`). This runs automatically before `dev`/`build`
(see `prebuild` in `package.json`), so `src/api/schema.ts` is always
regenerated fresh and is gitignored, not committed — there's no scenario
where it should differ from what `openapi.yaml` currently says.

`src/api/client.ts` wraps `fetch` with those generated types — no
client library on top (no `openapi-fetch`, no React Query) — there are
only four calls today; that's worth reconsidering once there's enough
call-site boilerplate for one to pay for itself, not preemptively.

## Commands

```sh
npm install         # install dependencies
npm run dev          # regenerate API types, then start the dev server
npm run build        # regenerate API types, type-check (tsc -b), production build
npm run generate:api # regenerate src/api/schema.ts from ../api/openapi.yaml by hand
npm run lint         # oxlint
npm run format       # prettier --write
```

`npm run format:check` and the above are what CI runs — see
[`../.github/workflows/ci.yml`](../.github/workflows/ci.yml) and the
root [`Makefile`](../Makefile)'s `web-*` targets.

The control plane must actually be running (with Postgres — see
[`../docs/DEVELOPMENT.md`](../docs/DEVELOPMENT.md)'s "Running the full
stack" section) for the dashboard to show real data; without it, the
node/workload lists show their fetch-error state.

## Linting

[`oxlint`](https://oxc.rs) is used instead of ESLint for speed. Type-aware
lint rules (via `oxlint-tsgolint`) are worth adding once there's real
component logic complex enough to benefit — arguably true now that
Phase 3's screens exist, worth revisiting.

## A known dependency-resolution quirk

`openapi-typescript@7.13.0` declares a peer dependency on
`typescript@^5.x`; this project uses `~6.0.2`. That's a peer-range lag
on the tool's part, not a real incompatibility — verified by actually
generating types from `../api/openapi.yaml` and type-checking the
output under this project's TypeScript version, not just by installing
successfully. `.npmrc` sets `legacy-peer-deps=true` so both `npm
install` and `npm ci` (what CI runs) resolve past it consistently.
