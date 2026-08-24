# CLAUDE.md — static-server

Instructions for Claude Code. See [AGENTS.md](AGENTS.md) for general agent context
(layout, commands, conventions, variable tables).

## Project identity

- **Module:** `github.com/meddleware-org/static-server`
- **License:** BSD Zero Clause (0BSD)
- **Language:** Go 1.24+, stdlib-only
- **Runtime base:** `scratch` (zero OS footprint)

This is a standalone OSS project. When extracted from any monorepo, it has no
external dependencies and no workspace-level configuration to inherit.

## Architecture rules

Two invariants must hold at all times:

1. **Cascade filesystem** — `cascadeFS` resolves files in `SERVE_DIR` first, then
   `FALLBACK_DIR`. Do not flatten this into a single `http.Dir`. The cascade is the
   feature that makes partial volume overlays work.

2. **Directory listing disabled** — `safeDir` wraps the cascade and returns
   `os.ErrNotExist` for directories without `index.html`. Do not bypass or remove
   `safeDir`. Any refactor that separates `safeDir` from the `http.FileServer` call
   will silently re-enable directory listing.

## Code style

- **Go doc on every symbol** — exported and unexported. Comment must start with the
  symbol name and be a complete sentence. Single-line comments are fine for simple
  items; multi-line for anything with behaviour worth explaining.
- **No external imports** — `go.mod` has no `require` stanzas. If you need a
  helper, write it or use stdlib.
- **Tests use real HTTP** — `httptest.NewServer` + `http.Get`, not mocks. The handler
  stack is constructed via `newHandler(fs http.FileSystem)` in test helpers; use this
  for any new test.
- **Single file** — all logic stays in `main.go`. Do not create additional `.go` files
  in the root package unless the file grows significantly beyond 500 lines.

## Documentation discipline

Any change that affects runtime behaviour, configuration, or endpoints must be reflected
in all of the following before the task is considered complete:

- `public/llms.txt` — machine-readable API reference served at `/llms.txt`
- `llms.txt` (repo root) — project-level AI/bot context
- `CHANGELOG.md` — under `[Unreleased]` or the relevant version entry
- `.env.example` — if a new configurable value is added
- `README.md` — if a user-visible feature or configuration changes
- `public/index.html` — API reference tables embedded in the default page

## What not to do

- Do not add `require` stanzas to `go.mod`.
- Do not add a shell, package manager, or OS layer to the container.
- Do not weaken `safeDir` (directory listing must stay disabled).
- Do not remove security headers from `secureHeaders` or bypass it for any route.
- Do not move accounting or business logic into the HTML page.
- Do not add a hidden daemon, background process, or off-container automation assumption.
- Do not use `//nolint` directives without an explicit justification comment.
- Do not amend published commits — create new commits instead.
