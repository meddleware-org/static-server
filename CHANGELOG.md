# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

## [0.1.9] - 2026-10-19

### Security

- **The image ships a restrictive default `Content-Security-Policy`.** The Dockerfile now sets
  `CONTENT_SECURITY_POLICY` to the same-origin policy below (no inline script or style, no third-party
  origin, no plugins, no `<base>`; `frame-ancestors 'self'` matches `X-Frame-Options: SAMEORIGIN`):

  ```text
  default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'self'; form-action 'self'
  ```

  The Go code default stays empty, so the bare binary is unchanged, and the eleven consumer images that set
  their own `CONTENT_SECURITY_POLICY` replace it. **Behaviour change for out-of-tree operators of the stock
  image:** mounted content that uses inline scripts or styles, or CDN assets, is now blocked; set
  `CONTENT_SECURITY_POLICY` to your own policy, or to an empty string to send no CSP. The built-in default
  page loads under the policy (no inline script or style; `TestDefaultPageFitsDefaultCSP`). `CSP_NONCE` still
  adds its nonce to `script-src`. Pinned by `TestImageDefaultCSP`, `TestBinaryCSPDefaultIsUnset` and
  `scripts/smoke.sh` (the default header on a stock-image response, the override, and the empty-value opt-out).

## [0.1.8] - 2026-10-18

### Security

- **The 405 response carries the security headers.** `methodGuard` ran outside `secureHeaders`, so a
  rejected method (POST, PUT, ...) got only `nosniff` from `http.Error`: no `X-Frame-Options`,
  `Referrer-Policy`, HSTS, `Permissions-Policy` or CSP. `secureHeaders` now wraps `methodGuard`.
- **`CSP_NONCE` also covers `script-src-elem`**, which takes precedence for `<script>` elements, so an
  edge-injected script is no longer blocked by a policy that sets it. Only the first occurrence of a
  directive is touched, and a directive that is exactly `'none'` is left alone.
- **No `Cache-Control` on the SPA-fallback 404.** When `SPA_FALLBACK` is on and there is no `index.html`
  at all, a navigation route under `CACHE_IMMUTABLE_PREFIX` answered 404 with the immutable header set
  earlier; it is now removed.

### Fixed

- **`PRECOMPRESSED` negotiation.** `Accept-Encoding` is parsed into whole tokens with q-values: a coding
  refused with `q=0` (`br;q=0, gzip`) is no longer served, and a token that merely contains `br` or
  `gzip` no longer matches. Asset responses carry `Vary: Accept-Encoding` whether or not a compressed
  sibling is served, so a shared cache keys the identity variant too.
- Documentation: the package comment gave `FALLBACK_DIR` the image default (`/app/default`) as the
  binary's; `CLEAN_URLS`, `NOT_FOUND_PAGE`, `CSP_NONCE`, `STRICT_TRANSPORT_SECURITY` and
  `PERMISSIONS_POLICY` are now in `public/llms.txt`, `public/index.html` (served at `/llms.txt` and the
  default page), `llms.txt`, `.env.example`, `README.md` and `AGENTS.md`; workflow, `envsubst`, Go
  version and test-count slips corrected in README, AGENTS, `llms.txt`, CLAUDE.md, the Dockerfile,
  the Makefile and the workflow headers.

### Added

- `scripts/smoke.sh` and a `make smoke` target: run a built image the way a cluster does (image user
  `65534`, read-only root, no capabilities) and check the in-image `-healthcheck`, `/healthz`, `/version`,
  the default page, the security headers on a 200, 404 and 405, a CSP nonce, a 404 under the immutable
  prefix with no `Cache-Control`, dot-files and the method guard. Works with Docker or rootless Podman.
- The `Go CI` workflow gains an `image` job (build the Dockerfile, run the smoke test), so a Dockerfile or
  builder-digest regression fails the pull request, and the release gate (`verify`) runs it on the tag.
- Tests: no caching headers on error responses (pins the stdlib behaviour that
  `GODEBUG=httpservecontentkeepheaders=1` would undo), `Accept-Encoding` parsing and `Vary`, the nonce on
  `script-src-elem`, security headers on 405, and `TestDocsListEveryOption` (every environment variable
  read in `main.go` is documented in every reference surface).

## [0.1.7] - 2026-10-09

### Security

- Built with Go 1.26.9: govulncheck found eleven net/http and net/textproto advisories in 1.26.7 (GO-2026-6607 to GO-2026-6617). go.mod names the toolchain and the Dockerfile builder is pinned to the same patch and checks it.

## [0.1.6] - 2026-10-08

(0.1.5 was tagged but never published: its release gate caught a lint finding.)

### Security

- **Served paths are confined to the serve root.** `SERVE_DIR` and `FALLBACK_DIR` are opened with
  `os.OpenRoot`: a symlink that leaves the root is refused (404) instead of followed, so a stray link
  in a mounted volume can no longer expose files such as a service-account token. Links that stay
  inside the root keep working.
- **Dot-files and dot-directories are not served** (`/.env`, `/.git/config`, backups), except a leading
  `/.well-known/`.

### Changed

- The release workflow reuses the CI workflow (lint, race tests, govulncheck, Trivy filesystem scan) on
  the tagged commit and scans the pushed image before signing it.
- `go.mod` names `toolchain go1.26.7`, and the Dockerfile fails if the builder image's Go differs, so
  the binary is built with the toolchain govulncheck checked.
- The published `cosign verify` hint pins this repository's release workflow instead of accepting any
  GitHub Actions identity.

## [0.1.4] - 2026-10-03

### Added

- `CLEAN_URLS` (default `false`): an extensionless path that does not exist is served from
  `<path>.html`, so generated sites with clean links (VitePress `cleanUrls`) serve the right page on
  a direct visit or refresh instead of relying on an SPA fallback.
- `NOT_FOUND_PAGE` (default empty): for an unknown navigation route, serve this page with status
  `404` (ignored when `SPA_FALLBACK` is on; missing assets keep a plain 404).

### Changed

- Navigation handling (SPA fallback, clean URLs, 404 page) applies to `HEAD` as well as `GET`, so a
  `HEAD` reports the same status as the `GET`.
- CI pins `govulncheck` (v1.8.0) instead of installing `@latest`.

## [0.1.3] - 2026-10-01

### Added

- `CSP_NONCE` (default `true`): a fresh 128-bit `'nonce-…'` is appended to the CSP's `script-src` on
  every response. Cloudflare reads it and tags the inline script that JavaScript Detections (always
  on with Bot Fight Mode) injects, which a strict `script-src` otherwise blocks — without allowing
  `'unsafe-inline'`. Policies without `script-src` are unchanged.
- `STRICT_TRANSPORT_SECURITY` (default `max-age=31536000; includeSubDomains`) and
  `PERMISSIONS_POLICY` (default `camera=(), geolocation=(), microphone=(), payment=()`) security
  headers; setting either variable to an empty string omits its header.

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
