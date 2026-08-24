# AGENTS.md — static-server

Context file for AI coding agents (Copilot Workspace, Codex, Devin, and similar tools).
See [CLAUDE.md](CLAUDE.md) for Claude-specific guidance.

## Project overview

static-server is a single-binary Go HTTP server built into a scratch container image (~5 MB).
It serves static files from a configurable primary directory (`SERVE_DIR`) with automatic
fallback to a secondary directory (`FALLBACK_DIR`), enabling partial volume overlays in
Kubernetes without requiring operators to supply every file.

The entire server logic lives in `main.go`. There are no third-party Go dependencies; the
module is stdlib-only (`go.mod` has no `require` stanzas).

## Repository layout

| Path | Purpose |
| --- | --- |
| `main.go` | All server logic: cascade filesystem, graceful shutdown, middleware, endpoints |
| `main_test.go` | Full test suite — 15 tests covering all endpoints, middleware, and cascade behaviour |
| `go.mod` | Module declaration (`github.com/meddleware-org/static-server`); no external dependencies |
| `Dockerfile` | Multi-stage build: `golang:1.26-bookworm` builder → `scratch` runtime |
| `Makefile` | Build, test, and push targets; all registry coordinates and metadata are Make variables |
| `.env.example` | All configurable values with defaults and descriptions |
| `.dockerignore` | Excludes build artifacts and project files from the Docker build context |
| `.gitignore` | Standard Go gitignore |
| `LICENSE` | BSD Zero Clause (0BSD) |
| `SECURITY.md` | Vulnerability reporting policy |
| `CHANGELOG.md` | Version history following Keep a Changelog format |
| `llms.txt` | Repo-root machine-readable project description (llmstxt.org spec) |
| `public/index.html` | envsubst template — all identifying values are `${VAR}` placeholders |
| `public/style.css` | Stylesheet for the default landing page |
| `public/script.js` | Sets `<link rel="canonical">` dynamically |
| `public/llms.txt` | Machine-readable API reference served at `/llms.txt` |
| `public/robots.txt` | Crawler policy (`Allow: /`) |
| `.github/workflows/ci.yml` | CI: `go vet` + `go test -race` on push and pull requests |
| `.github/workflows/publish.yml` | Publish: multi-platform image push to three registries on version tag |

## Key concepts

### Cascade filesystem (`cascadeFS`)

Files are resolved from `SERVE_DIR` first; misses fall through to `FALLBACK_DIR`. Both
directories are wrapped in an `http.FileSystem` slice (type `cascadeFS`). The image ships
baked-in defaults at `/app/default` so operators can mount partial content at `/app/public`
without needing to supply every file. Setting `FALLBACK_DIR` to an empty string disables
the cascade.

### Directory listing prevention (`safeDir`)

`safeDir` wraps `cascadeFS` and returns `os.ErrNotExist` for any directory path that does
not contain an `index.html`. This prevents `http.FileServer` from emitting directory
listings. The check is performed against the full cascade filesystem (both directories).

### Graceful shutdown

`main` installs a `SIGTERM`/`SIGINT` handler that calls `srv.Shutdown` with a 10-second
context. In-flight requests drain before the process exits. This is critical for Kubernetes
rolling updates.

### `-healthcheck` flag

When invoked as `server -healthcheck`, the process dials `localhost:PORT/healthz`, exits 0
on success and 1 on failure. Used by the Dockerfile `HEALTHCHECK CMD` since scratch images
have no `wget` or `curl`.

### `index.html` template

`public/index.html` is an envsubst template. All identifying values use `${VAR}` placeholders.
The Dockerfile builder installs `gettext-base` and runs `envsubst` to render `/app/default/index.html`
at build time. The raw template is preserved at `/app/default/index.html.tpl` for operator
use in initContainers.

## Commands

```bash
# Testing (must pass before any commit)
go test -v -race ./...

# Static analysis (must be clean)
go vet ./...

# Run locally
go run .

# Build single-platform image
make build VERSION=v0.1.0

# Multi-platform build + push to all registries
make build-multi VERSION=v0.1.0

# Override metadata without editing any file
make build-multi VERSION=v0.1.0 \
  SITE_TITLE="My App" \
  VENDOR="My Org" \
  SOURCE_URL=https://github.com/my-org/my-app \
  OG_URL=https://my-app.example.com/
```

## Conventions

- **No external dependencies.** `go.mod` must stay stdlib-only. Do not add `require` stanzas.
- **Single file.** All server logic stays in `main.go`. Do not split into packages unless the
  file significantly exceeds 500 lines.
- **Test every behaviour change.** Add or update tests in `main_test.go` for any change to
  server behaviour. Use `newHandler(fs)` to build the full handler stack in tests.
- **Go doc on every symbol.** Every exported and unexported type, function, variable, and
  method must have a Go doc comment. Comments must start with the symbol name and be suitable
  for `go doc` output.
- **No directory listing.** `safeDir` enforces this; do not remove or weaken it.
- **Security headers on every response.** `secureHeaders` wraps the top-level mux; do not
  bypass it for any route.
- **Keep documentation in sync.** Any API or configuration change must be reflected in
  `public/llms.txt`, `llms.txt`, `CHANGELOG.md`, `.env.example`, and `README.md`.

## Runtime environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | TCP listen port |
| `SERVE_DIR` | `/app/public` | Primary serve directory; mount operator content here |
| `FALLBACK_DIR` | `/app/default` | Fallback directory; empty string disables cascade |

## index.html template variables

Substituted at build time by `envsubst`; also usable at deploy time via initContainer.
The raw template is at `/app/default/index.html.tpl` inside the image.

| Variable | Default | Used in |
| --- | --- | --- |
| `SITE_TITLE` | `static-server` | `<title>`, `<h1>`, og:title, twitter:title, JSON-LD name |
| `DESCRIPTION` | (see Dockerfile) | meta description, og:description, twitter:description, JSON-LD description |
| `SITE_KEYWORDS` | (see Dockerfile) | meta keywords |
| `VENDOR` | `Meddleware` | meta author, JSON-LD author name |
| `SITE_AUTHOR_URL` | `https://meddleware.co.uk` | JSON-LD author URL |
| `OG_URL` | `` (empty) | og:url; set to deployment URL via initContainer for social sharing |
| `SOURCE_URL` | `https://github.com/meddleware-org/static-server` | body links, JSON-LD codeRepository |
| `IMAGE_URL` | `https://quay.io/meddleware-org/static-server` | JSON-LD downloadUrl (with https scheme) |
| `IMAGE_REF` | `quay.io/meddleware-org/static-server` | docker run and Kubernetes YAML examples |
| `VERSION` | `dev` | JSON-LD version |
| `SCHEMA_LICENSE_URL` | `https://opensource.org/licenses/0BSD` | JSON-LD license URL |

## Registry coordinates (Makefile / GitHub Actions)

| Variable / Secret | Default | Description |
| --- | --- | --- |
| `QUAY_REGISTRY` / `QUAY_NAMESPACE` | `quay.io` / `meddleware-org` | quay.io registry and namespace |
| `DOCKERHUB_REGISTRY` / `DOCKERHUB_NAMESPACE` | `docker.io` / `meddleware` | Docker Hub |
| `PRIVATE_REGISTRY` / `PRIVATE_NAMESPACE` | `registry.meddleware.co.uk` / `meddleware-org` | Self-hosted registry |
| `secrets.QUAY_USERNAME` / `secrets.QUAY_TOKEN` | — | quay.io credentials |
| `secrets.DOCKERHUB_USERNAME` / `secrets.DOCKERHUB_TOKEN` | — | Docker Hub credentials |
| `secrets.PRIVATE_REGISTRY_USERNAME` / `secrets.PRIVATE_REGISTRY_TOKEN` | — | Self-hosted credentials |
