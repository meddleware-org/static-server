# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- SPA hosting: `SPA_FALLBACK` (default `false`) serves `index.html` (200) for unknown
  navigation routes (extensionless paths) so client-side routing and deep-link refresh
  work; asset requests (paths with an extension) still 404 so broken builds are not masked.
- `CACHE_IMMUTABLE_PREFIX` (default empty) — files under this prefix get
  `Cache-Control: public, max-age=31536000, immutable` (for fingerprinted assets, e.g.
  Vite's `/assets/`); `index.html` and navigation routes get `no-cache`.
- `CONTENT_SECURITY_POLICY` (default unset) — sets the `Content-Security-Policy` response
  header when provided.
- `PRECOMPRESSED` (default `false`) — serves a sibling `.br`/`.gz` asset when the client
  accepts that encoding and the file exists, preserving the original `Content-Type` and
  adding `Vary: Accept-Encoding`.
- Correct MIME types on scratch (no `/etc/mime.types`): `.wasm` → `application/wasm`, plus
  `.mjs`, `.js`, `.css`, `.json`, `.map`, `.svg`, `.webp`, `.woff2`, `.ico`.
- `405 Method Not Allowed` (with `Allow: GET, HEAD`) for non-GET/HEAD requests.

### Changed

- Tightened default `Referrer-Policy` from `no-referrer-when-downgrade` to
  `strict-origin-when-cross-origin`. The new value sends only the origin (no path) on
  cross-origin navigations, which is the W3C-recommended default. No behaviour change
  for same-origin navigation or HTTPS→HTTPS cross-origin requests where the full URL
  was not meaningful.
- Request/lifecycle logging now uses `log/slog` (structured JSON) instead of `log.Printf`.
- The handler stack is now built by a shared `newHandler(config, fs)` in `main.go`, used by
  both the server and the tests (previously duplicated in the test file).

## [0.1.0] — 2026-08-18

### Added

- Initial release: stdlib Go static file server, scratch image, runs as UID 65534 (nobody).
- `PORT` environment variable — TCP listen port (default `8080`).
- `SERVE_DIR` environment variable — primary directory to serve static files from (default `/app/public`).
- `FALLBACK_DIR` environment variable — fallback directory served when a file is not found in `SERVE_DIR` (default `/app/default`; set to empty string to disable). Enables partial overlays: mount only the files you want to override in `SERVE_DIR` and let baked-in defaults serve from `FALLBACK_DIR`.
- Cascade filesystem (`cascadeFS`): resolves files in `SERVE_DIR` first; falls through to `FALLBACK_DIR` on miss. Baked-in default content lives in `/app/default` (never overwritten by operator volume mounts).
- `GET /healthz` — liveness/readiness probe endpoint; returns `200 ok`.
- `GET /version` — build-time version endpoint; returns the version string as plain text.
- `-healthcheck` flag — one-shot mode used by the `HEALTHCHECK` Dockerfile instruction on scratch images (no wget/curl available).
- Graceful shutdown on `SIGTERM`/`SIGINT` with a 10-second drain window.
- Security response headers on every response: `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, `Referrer-Policy: no-referrer-when-downgrade`.
- Per-request logging: method, path, status code, elapsed time.
- Directory listing disabled; directories without `index.html` return 404.
- OCI image spec labels injected at build time via `ARG` (`org.opencontainers.image.*`); forks override via `--build-arg` without editing the Dockerfile.
- `HEALTHCHECK` Dockerfile instruction for non-Kubernetes runtimes (Docker, Compose).
- Parameterised `Makefile` for local build, test, single-platform push, and multi-platform push to quay.io, Docker Hub, and a self-hosted registry.
- GitHub Actions CI workflow: `go vet` + `go test -race` on push and pull requests.
- GitHub Actions publish workflow: multi-platform image build and push on version tag (`v*`); all registry coordinates and OCI metadata flow from GitHub repository variables/secrets.
- `llms.txt` at repo root and served from the container (`/llms.txt`) for AI/LLM agent consumption.
- `public/robots.txt` — permits all crawlers.
- `public/index.html` — default landing page with full HTML metadata (Open Graph, structured data), API autodoc, and Kubernetes deployment examples.
- `public/index.html` as an envsubst template: all identifying values (`SITE_TITLE`, `DESCRIPTION`, `SITE_KEYWORDS`, `VENDOR`, `SITE_AUTHOR_URL`, `OG_URL`, `SOURCE_URL`, `IMAGE_URL`, `IMAGE_REF`, `VERSION`, `SCHEMA_LICENSE_URL`) are `${VAR}` placeholders substituted at build time via `gettext-base envsubst` in the builder stage. The rendered file lands at `/app/default/index.html`; the raw template is preserved at `/app/default/index.html.tpl` for initContainer use. All variables are individually overridable via `--build-arg` or Makefile variables without editing any file.
- `public/style.css` — extracted stylesheet for the default landing page; served independently from `FALLBACK_DIR` to enable partial ConfigMap overlays.
- `public/script.js` — sets `<link rel="canonical">` dynamically; served from `FALLBACK_DIR`.
- `LICENSE` — BSD Zero Clause (0BSD).
- `SECURITY.md` — vulnerability reporting policy.
- `.env.example` — documents all configurable environment variables with defaults.
