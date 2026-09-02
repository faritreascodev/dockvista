# Contributing to DockVista

Thanks for considering a contribution. This project favors small, focused
PRs over large ones — easier to review, easier to revert if needed.

## Development setup

```bash
git clone https://github.com/faritreascodev/dockvista.git
cd dockvista
make tidy   # go mod tidy + npm install
make dev    # API on :8080, Vite dev server with hot reload
```

You'll need a reachable Docker daemon (the default local socket works out
of the box).

## Project structure

Read the *Architecture* section of [README.md](README.md) first. In short:

- `internal/core` is framework-free — no `net/http`, no Docker SDK imports.
  If you're adding a use case, it goes in `internal/core/service`.
- `internal/adapters/docker` is the only package allowed to import
  `github.com/docker/docker`.
- `internal/adapters/httpapi` is the only package allowed to import
  `net/http` for the API surface.
- New Docker resources follow the existing pattern: a type in
  `internal/core/domain`, a narrow `ports.<Resource>Client` interface, an
  implementation in `internal/adapters/docker`, a `*Service` in
  `internal/core/service`, and handlers + DTOs in `internal/adapters/httpapi`
  — see the Images slice for a full example to copy.
- Frontend types in `web/src/types/domain.ts` must stay in sync with the
  Go DTOs in `internal/adapters/httpapi/dto.go` — there's no code
  generation between them yet, so update both by hand.

## Before opening a PR

```bash
make lint
make test
make build   # confirms the frontend embeds and the binary still compiles
```

All three must pass; CI runs the same checks.

## Commit style

Short, imperative subject lines (`Add container pause endpoint`, not
`Added` or `Adding`). Explain *why* in the body when the change isn't
self-evident from the diff.

## Reporting bugs / proposing features

Open a GitHub issue with:

- What you expected vs. what happened (for bugs), or the use case (for
  features)
- Go version, OS, and Docker version if relevant
- Steps to reproduce, if applicable

## Code of conduct

Be respectful. Assume good faith. Disagreements about implementation are
fine and expected — keep them focused on the code.
