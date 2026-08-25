# Ambud Web Dashboard

The primary way an operator uses Ambud — see
[`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) for why this isn't
a secondary/optional interface, and
[`../docs/ROADMAP.md`](../docs/ROADMAP.md) for which phase adds which
screen.

React + TypeScript + Vite. No state management library, UI kit, or
routing yet — those get chosen deliberately once there's more than a
placeholder page to justify them (see Phase 3 onward in the roadmap),
not speculatively now.

## Commands

```sh
npm install     # install dependencies
npm run dev     # dev server with hot reload
npm run build   # type-check (tsc -b) + production build
npm run lint    # oxlint
npm run format  # prettier --write
```

`npm run format:check` and the above are what CI runs — see
[`../.github/workflows/ci.yml`](../.github/workflows/ci.yml) and the
root [`Makefile`](../Makefile)'s `web-*` targets.

## Linting

[`oxlint`](https://oxc.rs) is used instead of ESLint for speed. Type-aware
lint rules (via `oxlint-tsgolint`) are worth adding once there's enough
real component logic to benefit from them — not yet, at placeholder-page
stage.
