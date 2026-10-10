# Security Audit — `static-server`

**Classification:** Internal security review (re-verified 2026-10-09 — awaiting external review)
**Project:** static-server — a minimal, stdlib-only Go HTTP static file server on a `scratch` image.

- **Serving.** A cascade filesystem (`SERVE_DIR` → `FALLBACK_DIR`), each root opened with `os.OpenRoot`
  (symlinks that leave it are refused) behind `noDotfiles` (dot-paths hidden except `/.well-known/`) and
  `safeDir` (directory listing disabled), and GET/HEAD only.
- **Routing modes.** SPA fallback, clean URLs, a 404 page, an immutable-prefix cache policy and
  precompressed siblings.
- **Headers.** Security headers on every response: CSP with an optional per-response script nonce,
  HSTS, Permissions-Policy, `nosniff`, `X-Frame-Options`, Referrer-Policy.
- **Operations.** `/healthz`, `/version`, a `-healthcheck` mode, graceful shutdown.
- **Scope of use.** It is the hosting runtime for **every** Meddleware web front end.

**Project type:** Go service + container image.
**Template:**

- AUDIT_TEMPLATE.md (2026-10-08)
- AUDIT_TEMPLATE_GO.md (2026-10-08) — names static-server
- AUDIT_TEMPLATE_IMG.md (2026-10-08) — names static-server
- AUDIT_TEMPLATE_VUE.md (2026-10-08) — hosting-header rows only (B.VUE-1, VUE-M8); see below

VUE-lens hosting rules (VUE-M8, CSP and HSTS on every hosting path) are assessed as the property this
runtime must make possible for its consumers; the rest of the VUE lens is the consumers' (their audits).

Not triggered: SUI, SUI_CLIENT, SEAL, WALRUS, AUTH, PROXY (it forwards nothing), OPS, TS, RUST, WORKERS,
SITE (the docs and dev sites are its consumers, audited separately), PLATFORM (the cluster is audited
separately).

**Deployment status:**

- Release `v0.1.7` = `e1a71c5` (2026-10-09) = HEAD of `main`; Publish run 37964221333 succeeded (full CI
  through the reusable workflow, Trivy image scan, cosign, SBOM and provenance attestations). Tags
  `v0.1.5` (never published: the release gate caught a lint finding; Docker Hub has no `0.1.5`) and
  `v0.1.6` (`86e1e70`) precede it.
- Docker Hub `meddleware/static-server:0.1.7` = index `sha256:2e227311…2379` (re-read from the registry
  2026-10-09; the same digest is pinned in `config/images.yaml`). Its referrers carry three Sigstore
  bundles (cosign signature, SBOM attestation, build provenance). `0.1.6` = `sha256:be51c4ee…`. quay.io
  was not queried from this sandbox; the Publish run pushed and scanned the quay.io image.
- **Consumers (all eleven pinned by digest to `0.1.7@sha256:2e227311…`):** access-gate-ui, dao-ui (retired
  from the cluster, still published), dashboard, dev, docs, landing, seal-ui, status-page,
  token-deployer-ui, treasury-ui, walrus-ui. Every consumer Dockerfile sets `CONTENT_SECURITY_POLICY`
  and `CACHE_IMMUTABLE_PREFIX`; the SPAs set `SPA_FALLBACK=true`. Deployed images were cosign-verified
  2026-10-09 (`bootstrap/images/verify-digests.sh`, 16/16 valid). None sets `PRECOMPRESSED`.
- Consumer pods set `automountServiceAccountToken: false` (every `post-bootstrap/*/base` deployment and
  the `k8s/base` status and registry pods).

**Review date:** 2026-10-03; re-verified 2026-10-09
**Reviewer:** Internal review
**Severity ceiling:** High.

- It serves every public front end, and its headers are those front ends' browser security policy
  (CSP, HSTS, framing).
- A path-confinement or header defect would affect every app at once.
- Realised ceiling: **Low** at the first pass (F1–F3), all three since RESOLVED; what remains open is
  Info.

**Status:** re-verified 2026-10-09 (first pass 2026-10-03). The CHANGELOG records an earlier internal
review ("audited", 2026-09-21) with no audit document in the repo; the 2026-10-03 pass was the first
corpus-format audit.

**Front matter (GO lens):**

| Field | Value |
| --- | --- |
| Go | `go 1.26.6` and `toolchain go1.26.9` (go.mod); CI via `go-version-file` (resolves the toolchain directive, so CI checks 1.26.9); the release image built by `golang:1.26.9-bookworm@sha256:d9c68c2c…` — the Dockerfile fails the build if the builder's `go env GOVERSION` differs from the `toolchain` line (F2) |
| Modules | none (stdlib only; `go.mod` has no `require`, so no `go.sum`) |
| Entry points | `/server` (listen `:$PORT`, default 8080); `/server -healthcheck` |
| Upstreams | none (the healthcheck dials `localhost:$PORT/healthz`) |

**Front matter (IMG lens):**

| Field | Value |
| --- | --- |
| Images | `quay.io/meddleware-org/static-server:0.1.7@sha256:2e227311…`; `docker.io/meddleware/static-server:0.1.7@sha256:2e227311…`; `registry.meddleware.co.uk/…` (best-effort, fails without credentials — maintainer item) |
| Base images | builder `golang:1.26.9-bookworm@sha256:d9c68c2c…`; runtime `scratch` |
| Runtime user | `USER 65534` in the Dockerfile (`nobody`, 65534:65534 via the generated `/etc/passwd`); consumer pods add `runAsNonRoot`, `runAsUser: 65534`, no privilege escalation, all capabilities dropped and `RuntimeDefault` seccomp (checked in the ten consumer deployments that run it: access-gate-ui, dashboard, dev, docs, landing, seal-ui, token-deployer-ui, treasury-ui, walrus-ui, status) |
| Runtime FS | `/app/default` (baked), `/app/public` (operator mount or consumer `COPY`); consumer pods set `readOnlyRootFilesystem: true` |
| Deployed by | consumers' images `FROM` this one by digest; digest source `config/images.yaml` |
| Build args | `VERSION`, `VENDOR`, `DESCRIPTION`, `SOURCE_URL`, `DOCUMENTATION_URL`, `IMAGE_URL`, `SITE_*`, `OG_URL`, `IMAGE_REF`, `SCHEMA_LICENSE_URL` — labels and default-page text; no secrets |

**Location:** `static-server/docs/audit/static-server-audit.md` (canonical: the repo-local audit). The
`docs/` directory is not yet committed in the repo.

> **Access note (2026-10-09):** read-only re-verification of `workspace/repos/static-server` at `main`
> (`e1a71c5`). No code, commit or push was made by this pass.

---

## Executive summary

`main.go` (662 lines) holds all the logic. There are 37 tests (`main_test.go`, 962 lines) over real
HTTP (`httptest`). Since the 2026-10-03 pass the three Low findings were fixed and released (0.1.6 and
0.1.7), the Go toolchain advisory found on 2026-10-09 was fixed (0.1.7), and the remaining Info items
were re-checked and given reasoned dispositions.

**What holds (verified):**

- **Stdlib only.** No third-party modules.
- **Paths stay in the serve root (F1, RESOLVED).** `rootedFS` opens each root with `os.OpenRoot`, so a
  symlink that leaves it is a 404; `noDotfiles` hides dot-paths except a leading `/.well-known/`.
- **Directory listing disabled.** `safeDir` wraps the whole cascade, and a directory without
  `index.html` returns 404.
- **Read-only.** GET and HEAD only (405 with `Allow` otherwise).
- **Headers on every response, including errors.**
  - Always: `nosniff`, `X-Frame-Options: SAMEORIGIN`, Referrer-Policy.
  - When configured: CSP with a per-response 128-bit nonce, HSTS (default one year,
    `includeSubDomains`), Permissions-Policy.
- **Traversal confined.** Encoded traversal never leaves the serve root (tested).
- **No immutable 404s.**
  - The 2026-10-03 probe refuted the hypothesis that a missing fingerprinted asset is served `404`
    with `Cache-Control: public, max-age=31536000, immutable`: since Go 1.23, `http.FileServer`'s
    error path deletes `Cache-Control`, `Content-Encoding`, `Etag` and `Last-Modified`.
  - The safety still depends on the toolchain and `GODEBUG` defaults, and no test pins it (S1 stays a
    suggestion; the grep of `main_test.go` finds no 404-under-prefix assertion).
- **Navigation handling.** SPA, clean-URL and not-found navigations override the immutable header
  with `no-cache`.
- **Server limits.** Read 15 s, write 30 s, idle 60 s timeouts; graceful 10-second shutdown on
  SIGTERM; JSON request logs without client IPs.
- **Release gate equals CI (F2, RESOLVED).** The `verify` job is the reusable Go CI workflow (lint,
  vet and race tests, govulncheck, Trivy filesystem scan) on the tagged commit; the pushed image is
  Trivy-scanned (fixable CRITICAL/HIGH) before cosign; `go.mod` names `toolchain go1.26.9` and the
  Dockerfile fails if the builder's Go differs. The gate has already stopped a release: `v0.1.5` was
  never published.
- **Verification pins the signer (F3, RESOLVED).** The release summary and SECURITY.md print an
  anchored identity regexp for this repository's `docker-publish.yml` on `v*` tags.
- **Image.**
  - Builder pinned by digest to the toolchain patch; static `CGO_ENABLED=0` binary on `scratch`;
    `USER 65534`.
  - Multi-arch.
  - Keyless cosign signature, SPDX SBOM attestation and GitHub build-provenance attestation (three
    Sigstore bundle referrers on the Docker Hub 0.1.7 index).
- **CI.** golangci-lint (pinned v2.13.0), `go vet`, race tests with coverage, govulncheck (pinned
  v1.8.0), a Trivy filesystem scan (vulns, misconfig, secrets), SHA-pinned actions, and Dependabot
  (weekly, grouped: docker and github-actions).
- **Measured (2026-10-09).** `go vet` clean; **37 tests pass with `-race`**; **coverage 91.9%**
  (local Go 1.27.1; CI runs 1.26.9).

**Findings and dispositions (F1–F11):** 4 RESOLVED (F1, F2, F3, F11), 4 ACCEPTED-RISK (F4, F5, F7,
F10), 1 ADJUDICATED (F9), 1 DEFERRED (F6, to the pre-mainnet gate), 1 Positive (F8). None above Info is
unresolved.

- **F4 — CSP is off by default and the nonce touches only `script-src`.** Info; every one of the eleven
  consumers sets a CSP, none uses `script-src-elem` or `'unsafe-inline'`. Whether the image should ship a
  strict default is OQ2.
- **F5 — precompressed negotiation ignores `q=0`.** Latent: no consumer sets `PRECOMPRESSED`.
- **F6 — docs-discipline drift.** Still present (five options missing from the served `llms.txt` and
  default page); docs-only, pre-mainnet gate.
- **F7, F10 — minor runtime notes; no image build in PR CI.** Accepted with reasons.

**Posture:**

- A small, careful server with strong CI and release attestations; the path-confinement, release-gate
  and verification-guidance gaps found on 2026-10-03 are closed in code and released.
- The rare cache-header trap is closed by the Go stdlib (worth a test: S1).
- What remains is Info-level hardening and documentation, plus the external review (pre-mainnet).

---

## Threat model / trust boundaries

| Actor / source | Controls | Can do | Bounded by |
| --- | --- | --- | --- |
| Anonymous client (through Cloudflare and ingress) | method, path, query, headers (`Accept-Encoding`, `If-None-Match`, `Range`) | Request any path; probe for files | GET/HEAD only; `os.OpenRoot` confinement and `noDotfiles` (F1, RESOLVED); `safeDir`; server timeouts. **`q=0` ignored (F5, latent)** |
| Consumer image build (`COPY dist /app/public`) | the content tree and the env (CSP, prefixes, modes) | Ship any files and headers (a stray `.env` is now a 404) | consumer audits; build-time CSP (verified present in all eleven consumers, 2026-10-09) |
| Operator / cluster (volume mounts, env) | `SERVE_DIR`/`FALLBACK_DIR` contents, including symlinks; header values | Disable headers (`""`); serve any non-dot file inside the root. A symlink out of the root is refused (F1) | SECURITY.md excludes operator content. `os.OpenRoot`; consumers set `automountServiceAccountToken: false` |
| Edge (Cloudflare) | caching, injected scripts | Cache responses; inject scripts using the CSP nonce | `no-cache` on HTML and navigations; immutable only on the prefix; error responses stripped of caching headers (stdlib) |
| Image consumers verifying signatures | the identity they accept | Accept an image signed by a wrong workflow | anchored identity regexp in the release summary and SECURITY.md (F3, RESOLVED). The cluster `ClusterImagePolicy` pins the issuer and any `meddleware-org` workflow, in `warn` mode (OQ3) |
| Builder base image / Go toolchain | the compiled binary and stdlib | Ship a vulnerable stdlib | digest pin; `toolchain go1.26.9` checked in the Dockerfile; govulncheck and Trivy at release (F2, F11, RESOLVED) |

### Image supply chain (IMG lens)

| Actor / asset | Power | Bounded by |
| --- | --- | --- |
| Base-image publisher (`golang`) | the compiler and stdlib | digest pin (IMG-M1); toolchain check against `go.mod` |
| Build context | `go.mod`, `*.go`, `public/` | `.dockerignore` (excludes `.env*`, tests, docs, `.github`); selective `COPY` |
| Build args | label and default-page text (substituted into HTML with `sed`) | operator-controlled. The Dockerfile notes the `|` and `&` limits; no HTML escaping (F7); no secrets |
| CI publish job | push, sign, attest | SHA-pinned actions; OIDC; image scan before sign (F2); the image is pushed before it is scanned (F9) |
| Registry | the manifest for a tag | consumers pin by digest (all eleven verified 2026-10-09); `verify-digests.sh` cosign-checks each pinned digest |

---

## Severity scale

Critical / High / Medium / Low / Info / Positive.

## Scope

**In scope (HEAD and release `v0.1.7` = `e1a71c5`; first pass was HEAD `f596636`, release `v0.1.4` = `448dc45`):**

- `main.go`, `main_test.go`, `go.mod`
- `Dockerfile`, `.dockerignore`, `Makefile`
- `public/*` (default page, `script.js`, `style.css`, `llms.txt`, `robots.txt`)
- `.github/workflows/{go-ci,docker-publish}.yml`, `.github/dependabot.yml`
- `README.md`, `CLAUDE.md`, `AGENTS.md`, `SECURITY.md`, `CHANGELOG.md`, `llms.txt`, `.env.example`

**Cross-repo evidence (read-only):**

- consumers' Dockerfiles: base pin, CSP, `CACHE_IMMUTABLE_PREFIX`, `SPA_FALLBACK`; consumer pod specs
  (`post-bootstrap/*/base`, `k8s/base/apps/status`): security context, `automountServiceAccountToken`;
- `config/images.yaml`, `bootstrap/images/verify-digests.sh`, the cluster `ClusterImagePolicy`;
- Go stdlib `net/http/fs.go` `serveError` (first pass, GOROOT 1.26.6);
- the Docker Hub registry API (0.1.7 and 0.1.6 digests, referrers) and the public GitHub Actions run
  history for the repository (read-only).

**Out of scope:** consumers' content and CSP values (their audits); Cloudflare and ingress
configuration.

**Environment / commands:**

| Command | Result |
| --- | --- |
| `go vet ./...` (2026-10-09, local Go 1.27.1) | clean |
| `go test -race -cover ./...` (2026-10-09) | **ok** (37 tests), **coverage 91.9%** |
| 2026-10-03 scratch probe (deleted afterwards; code since changed) | missing `/assets/*.js` → 404 with **no** `Cache-Control` (stdlib strips it); `/assets/` and `/assets/sub` → SPA index, `no-cache`; `/.env`, `/.git/config` → 200 and symlink `/link.txt` → `/etc/hostname` served (**both closed in 0.1.6, F1**); `Accept-Encoding: br;q=0, gzip` → `br` and identity variant without `Vary` (F5, unchanged in the code); CSP with `script-src-elem` → nonce added to `script-src` only (F4, unchanged) |
| govulncheck / golangci-lint | not run locally (not installed; `vuln.go.dev` was 403 from the sandbox on 2026-10-03). Both ran green in the Publish run for `v0.1.7` (run 37964221333, jobs "Vulnerabilities (govulncheck)" and "Lint (golangci-lint)") |
| GitHub Actions history (public API) | `Publish (Docker)` for `v0.1.7` success; Go CI on `main` failed at 2026-10-09 16:51Z and passed at 17:09Z on `e1a71c5` (consistent with the 1.26.7 advisories recorded in the 0.1.7 CHANGELOG entry); the Publish run's self-hosted job fails at registry login (best-effort job, maintainer item) |
| Docker Hub API | `0.1.7` index `sha256:2e227311…2379` (matches `config/images.yaml`); `0.1.6` = `sha256:be51c4ee…`; `0.1.5` → 404; referrers of 0.1.7: three `application/vnd.dev.sigstore.bundle.v0.3+json` |
| Cluster manifests (read) | 18 workload manifests set `automountServiceAccountToken: false`; the ten consumer deployments checked set `runAsNonRoot`, `readOnlyRootFilesystem`, no privilege escalation, dropped capabilities and `RuntimeDefault` seccomp |

Not run in this pass: the cosign identity check against the registry (the `verify-digests.sh` result of
2026-10-09, 16/16 valid, is the workspace's evidence), quay.io from this sandbox.

The repo was not modified by this pass (read-only; the first pass's probe was deleted).

---

## Findings

### F1 — Served paths are not confined to the serve root: symlinks escape, and dotfiles and VCS metadata are served

**Severity:** Low (needs operator-supplied content; GO-M3)   **Disposition:** RESOLVED (0.1.6, `192b01f`)
**Where:** as found: `main.go` `http.Dir(cfg.ServeDir)`, `http.Dir(cfg.FallbackDir)`, `cascadeFS`, `safeDir`.
Now: `main.go:168-170` (`rootedFS(cfg.ServeDir)`, `rootedFS(cfg.FallbackDir)`), `:468-509` (`rootedFS`,
`escapeIsNotFound`, `noDotfiles`), `:254` (`safeDir{noDotfiles{fs}}`); SECURITY.md scope ("Security issues arising from content mounted at
`SERVE_DIR`" excluded).

**Issue (probe-verified):**

- **Symlinks.** `http.Dir` blocks `..` traversal but **follows symlinks**. A symlink
  `SERVE_DIR/link.txt → /etc/hostname` was served with status 200.
- **Dotfiles and VCS metadata.** `/.env` and `/.git/config` were served with status 200. There is no
  deny-list for dot-segments, `.git`, `.env*` or backup files (`*~`, `*.bak`).
- **Why it can matter here.** Nothing in today's consumer images contains such files: they copy a
  Vite or VitePress `dist`, and the `scratch` root holds only `/etc/passwd`, `/server` and
  `/app/*`. But:
  - operators can mount volumes (the README documents ConfigMap overlays);
  - a consumer's build could ship a stray `.env` from `public/`;
  - a misplaced symlink (or a mounted volume carrying one) exposes whatever the container can read,
    including service-account tokens at `/var/run/secrets/kubernetes.io/serviceaccount/token` if
    automounted.

**Impact:** a misconfiguration becomes a file disclosure through a public host. GO-M3 ("confine
served paths to the serve root") is met for path syntax but not for symlinks.

**Remediation / evidence:**

- **Fixed in 0.1.6** (`192b01f`, "Confine paths to the serve root (OpenRoot), hide dot-files …"; `0.1.5`
  was tagged but never published — its release gate caught a lint finding, fixed in `86e1e70`).
  - `rootedFS` opens each root with `os.OpenRoot` and serves `http.FS(root.FS())`: a symlink that
    leaves the root is refused and reported as 404 by `escapeIsNotFound` (not the 500 that
    `http.FileServer` would give), while links inside the root (ConfigMap `..data`) keep working. A
    root that cannot be opened serves nothing (404) instead of stopping the server.
  - `noDotfiles` makes any path segment starting with `.` a 404, except a leading `/.well-known/`.
    `fileHandler` composes `safeDir{noDotfiles{fs}}`, so the cascade and the existence checks both
    see it.
  - Tests that pin it (`main_test.go`): `TestSymlinkEscapeRefused`, `TestDotfilesHidden`,
    `TestRootedFSMissingDirServesNothing`. CLAUDE.md invariant 2 and SECURITY.md "Invariants" state it.
- **Consumers (OQ1 context):** every consumer pod sets `automountServiceAccountToken: false`, so the
  service-account token path named above no longer exists in the container (checked in all
  `post-bootstrap/*/base` and `k8s/base` deployments, 2026-10-09). The ConfigMap overlay in the
  README is the only documented volume use.

### F2 — The release gate is weaker than CI, and the shipped Go toolchain differs from the checked one

**Severity:** Low   **Disposition:** RESOLVED (0.1.6 `192b01f`; toolchain bumped in 0.1.7 `e1a71c5`)
**Where:** as found: `.github/workflows/docker-publish.yml` (`verify`: `go vet` + `go test -race` only),
`Dockerfile:4` (`golang:1.26-bookworm@sha256:6ef6e30f…`), `go.mod` (`go 1.26.6`).
Now: `docker-publish.yml` (`verify` → `uses: ./.github/workflows/go-ci.yml`; "Scan the published image"
step before "Sign images"), `go-ci.yml` (`workflow_call`), `Dockerfile:4` and the `RUN grep -q "^toolchain …"`
check, `go.mod` (`toolchain go1.26.9`).

**Issue:**

- **Tag-time checks.** A `v*` tag triggers only `docker-publish.yml`, whose `verify` job runs `vet`
  and race tests. golangci-lint, govulncheck and the Trivy filesystem scan never run against the
  tagged commit; they ran on whatever branch pushes preceded it.
- **No image scan.** The pushed image is not scanned (Trivy or Grype image scan; IMG Section C).
- **Toolchain mismatch.**
  - CI uses `go-version-file: go.mod`, i.e. Go **1.26.6**, so govulncheck evaluates the 1.26.6
    stdlib.
  - The release binary is built by the pinned `golang:1.26-bookworm` digest, whose SBOM shows
    **`stdlib go1.26.7`**.
  - The two can diverge in either direction. A builder digest left on an older patch would ship a
    stdlib with known CVEs that CI's govulncheck (on `go.mod`'s version) would not flag.

**Impact:** a release can ship a binary whose stdlib was never vulnerability-checked, and lint or
scan regressions can reach a tag.

**Remediation / evidence:**

- **Release gate = CI.** `go-ci.yml` gained a `workflow_call` trigger and `verify` calls it, so lint,
  vet and race tests, govulncheck and the Trivy filesystem scan run on the tagged commit; both publish
  jobs `need: verify`. The Publish run for `v0.1.7` (run 37964221333) shows all four verify jobs green
  before the publish job; the `0.1.5` tag shows the gate refusing a release.
- **Image scan before signing.** The "Scan the published image" step (Trivy, `exit-code: 1`,
  CRITICAL/HIGH, `ignore-unfixed`) runs on the pushed digest and precedes cosign (see F9 for the
  push-then-scan order).
- **Toolchain aligned.** `go.mod` names `toolchain go1.26.9` (setup-go reads it, so govulncheck checks
  that patch) and the Dockerfile builder is `golang:1.26.9-bookworm@sha256:d9c68c2c…` with a build step that
  fails unless `go env GOVERSION` equals the `toolchain` line. A Dependabot bump of the builder
  digest to another patch therefore fails the release build closed (see F10 for the PR-time gap).
- Evidence that it matters: the 0.1.7 CHANGELOG entry records eleven stdlib advisories on Go 1.26.7
  that govulncheck found (F11).

### F3 — The published verification hint accepts any GitHub Actions identity

**Severity:** Low   **Disposition:** RESOLVED (0.1.6, `192b01f`)
**Where:** as found: `.github/workflows/docker-publish.yml` step summary (`cosign verify
--certificate-identity-regexp '.*' …`). Now the "Summary" step of `publish-public` and SECURITY.md
"Invariants".

**Issue:**

- `--certificate-identity-regexp '.*'` accepts a keyless signature from **any** GitHub Actions
  workflow in any repository.
- Anyone can sign an image they control with their own workflow. That proves nothing about *this*
  repository's release workflow having built it.
- The hint is printed on every release summary, the place operators copy it from.

**Impact:**

- Consumers (and the platform's admission policy, if it follows this hint) would accept a
  substituted image as "signed".
- IMG-M7's signature is only as strong as the identity it is verified against.

**Remediation / evidence:**

- **Fixed in 0.1.6** (`192b01f`): the release summary now prints
  `--certificate-identity-regexp '^https://github.com/meddleware-org/static-server/\.github/workflows/docker-publish\.yml@refs/tags/v'`
  with the GitHub OIDC issuer, and SECURITY.md carries the same command (against the quay.io image).
  The README does not repeat it. The workspace's `bootstrap/images/verify-digests.sh` verifies each
  pinned digest against a per-image `https://github.com/meddleware-org/<name>/` identity prefix
  (16/16 valid, 2026-10-09).
- **Cluster admission (OQ3):** the `ClusterImagePolicy` in `k8s/base/infrastructure/policy-controller`
  pins the GitHub issuer and a `meddleware-org/<any repo>/.github/workflows/…` regexp (any workflow of
  any org repository, not this one) and is in `warn` mode. That is a platform item (not this repository);
  see the PLATFORM audit.

### F4 — CSP is off by default, and the nonce handling has edge cases

**Severity:** Info   **Disposition:** ACCEPTED-RISK (default pending OQ2)
**Where:** `main.go:111-112` (`CONTENT_SECURITY_POLICY` default `""`; `CSP_NONCE` default `true`),
`secureHeaders` (`:551`), `withScriptNonce` (`:587`). Unchanged since the first pass.

**Issue / Impact:**

- **Off by default.** No CSP is sent unless the operator sets one, including on the image's own
  default page. Every current consumer sets one (verified), but VUE-M8's "every hosting path"
  depends on configuration, not the runtime.
- **Only `script-src` gets the nonce.** A policy using `script-src-elem` (which takes precedence for
  `<script>` elements) still blocks edge-injected scripts (probe).
- **The nonce disables `'unsafe-inline'`.** Under CSP2+ a nonce in `script-src` makes browsers
  ignore `'unsafe-inline'` there. No consumer uses `'unsafe-inline'` in `script-src` today (they use
  `'self'` plus hashes), but a future one would see its inline scripts break silently.
- **Nonce on every response.** The nonce is generated for every response, including assets, and is
  reused if an edge caches an HTML response with its header. This matters only if there is an HTML
  injection point, and this server has none.

**Remediation / evidence:** re-checked 2026-10-09; the code is unchanged and no fix is applied.

- **Why accepted.** Every consumer image sets `CONTENT_SECURITY_POLICY` in its Dockerfile (all eleven,
  grep of `repos/*/Dockerfile`); their `script-src` values are `'self'`, `'self'` plus `'wasm-unsafe-eval'`
  or `'self'` plus two `sha256-` hashes (VitePress, checked at build time). None uses `script-src-elem`
  or `'unsafe-inline'`, so the nonce edge cases cannot bite today. The nonce is a random header value
  that lets only markup carrying it run, and the server has no HTML injection point.
- **Open choice.** Whether the image itself should ship a strict default CSP (consumers override) is
  OQ2, a maintainer decision.
- **If a consumer changes.** Add `script-src-elem` handling and a `text/html`-only nonce, and document
  the `'unsafe-inline'` interaction, before any consumer relies on them.

### F5 — Precompressed negotiation ignores `q=0`, and only one variant carries `Vary`

**Severity:** Info (latent: no consumer sets `PRECOMPRESSED`)   **Disposition:** ACCEPTED-RISK
**Where:** `main.go:385-425` (`tryPrecompressed`). Unchanged since the first pass.

**Issue / Impact (probe-verified):**

- `strings.Contains(ae, "br")` serves `br` to `Accept-Encoding: br;q=0, gzip`, a client that
  explicitly refused it. Substring matching would also match tokens containing `br` or `gzip`.
- `Vary: Accept-Encoding` is added only when a compressed sibling is served. The identity response
  for the same URL has no `Vary`, so a shared cache can store it unkeyed. RFC 9110 expects `Vary` on
  every variant.

**Remediation / evidence:** re-checked 2026-10-09: still `strings.Contains(ae, enc.name)` and `Vary` only
on the compressed branch. Accepted because the feature is off by default and no consumer enables it (no
`PRECOMPRESSED` in any consumer Dockerfile or `post-bootstrap` base manifest). The exact fix stays: parse
`Accept-Encoding` tokens with q-values (reject `q=0`); set `Vary: Accept-Encoding` on every response for
which a compressed sibling may exist; add tests. It MUST land before any consumer sets `PRECOMPRESSED`.

### F6 — Docs-discipline drift (CLAUDE.md requires every option in all reference surfaces)

**Severity:** Info   **Disposition:** DEFERRED (docs-only; the pre-mainnet gate "docs discipline restored" in Section D)
**Where:** CLAUDE.md "Documentation discipline" (`public/llms.txt`, `llms.txt`, CHANGELOG,
`.env.example`, README, `public/index.html`).

| Option | README | `llms.txt` | `public/llms.txt` | `public/index.html` | `.env.example` |
| --- | --- | --- | --- | --- | --- |
| `CLEAN_URLS` | ✓ | ✗ | ✗ | ✗ | ✓ |
| `NOT_FOUND_PAGE` | ✓ | ✗ | ✗ | ✗ | ✓ |
| `CSP_NONCE` | ✓ | ✓ | ✗ | ✗ | ✗ |
| `PERMISSIONS_POLICY` | ✓ | ✓ | ✗ | ✗ | ✗ |
| `STRICT_TRANSPORT_SECURITY` | ✓ | ✓ | ✗ | ✗ | ✗ |

Also:

- the `main.go` package comment says `FALLBACK_DIR` defaults to `/app/default`, while `loadConfig`
  defaults to `""` (the image's `ENV` sets `/app/default`);
- `go-ci.yml`'s header comment names `publish.yml` (actual `docker-publish.yml`);
- `docker-publish.yml`'s header comment says the SBOM and provenance are "attached by BuildKit", while
  `sbom: false` is set and the SBOM comes from the anchore step as a cosign attestation;
- ~~SECURITY.md lists no invariants~~ — fixed in 0.1.6: SECURITY.md now has an "Invariants" section.

The served `/llms.txt` and default page are the machine-readable API reference bots consume. They
currently omit five of the server's options.

**Remediation / evidence:** re-checked 2026-10-09 by grep of the five option names in each surface: the
table above is unchanged (the 0.1.5–0.1.7 changes touched CHANGELOG, CLAUDE.md, SECURITY.md and
Dockerfile, not `llms.txt`, `public/llms.txt`, `public/index.html` or `.env.example`). Not fixed here
(no doc edits in this pass). Exact remediation: add the five options to `public/llms.txt`,
`public/index.html` and (where missing) `llms.txt` and `.env.example`; correct the three comment
slips; S3 would stop it recurring. Tracked by the pre-mainnet gate in Section D.

### F7 — Minor runtime notes

**Severity:** Info   **Disposition:** ACCEPTED-RISK

- **Write timeout.** `WriteTimeout: 30s` cuts any response not finished in 30 s. Behind Cloudflare
  this is fine, but direct or slow clients downloading large assets (the 11.6 MB ui legal PDF, wasm
  bundles) can be truncated.
- **Edge-case header.** `serveIndex`'s `http.NotFound` path (no `index.html` at all) keeps any
  `Cache-Control: immutable` set earlier for paths under the prefix. That needs a broken
  deployment.
- **SBOM version.** The SBOM lists the module version as `UNKNOWN` (no VCS info in the Docker build
  context). Pass `-ldflags -X` (already done for `main.version`) and set
  `-buildvcs=false`/annotations, or record the version in the SBOM.
- **Default-page build args.** They are substituted into HTML (including the JSON-LD block) by `sed`
  without HTML or JSON escaping. Operator-controlled and documented (`|`/`&` note), but a `</script>`
  or quote in a build arg would break the page.

**Remediation / evidence:** re-checked 2026-10-09; the code and Dockerfile are unchanged on these points.
Accepted with reasons: all public traffic reaches the server through Cloudflare (which buffers
responses), so a 30 s write cap only affects a direct slow client; the immutable-header edge case needs
a deployment with no `index.html` at all; the SBOM module field is cosmetic (the stdlib and Go version
are listed); build args are set by the maintainer's CI and the Dockerfile documents the `|`/`&` limits.

### F8 — Positive: stdlib-only, confined, listing-safe, method-guarded, headers on every response; release attestations verified

**Severity:** Positive

- **Code.** `go.mod` has no `require`. All logic is in one file with complete doc comments
  (CLAUDE.md).
- **`safeDir`** wraps the entire cascade, so directory listing is impossible. `exists()` uses the
  same semantics. Since 0.1.6 each root is an `os.OpenRoot` and dot-paths are hidden (F1).
- **Methods and headers.** `methodGuard` returns 405 for anything but GET/HEAD. `secureHeaders` runs
  before routing, so headers are present on 404, 405 and health responses.
- **Cache policy.**
  - Immutable only under the configured prefix.
  - `no-cache` for HTML, directories and navigations.
  - SPA, clean-URL and not-found pages force `no-cache`.
  - Error responses lose caching headers (Go ≥ 1.23 `serveError`), so a missing fingerprinted asset
    is never cached for a year (probe).
- **Content types.** Explicit types for `.wasm`, `.js`, `.mjs`, `.css`, `.svg` and fonts on a
  `scratch` image without `/etc/mime.types`.
- **CSP nonce.** 128-bit `crypto/rand`, and a panic on RNG failure.
- **Tests.** 37 real-HTTP tests (traversal, encoded traversal, symlink escape, dot-files, listing,
  cascade, SPA, clean URLs, 404 page, headers, precompressed, nonce), race-clean, 91.9% coverage
  (2026-10-09).
- **Image and release.**
  - `scratch`, nonroot `65534`, a digest-pinned builder checked against the `go.mod` toolchain,
    `CGO_ENABLED=0 -trimpath -s -w`, and a `-healthcheck` mode.
  - Multi-arch; keyless cosign, SLSA provenance (BuildKit `mode=max` + `attest-build-provenance`) and
    an SPDX SBOM attestation (three Sigstore bundle referrers on Docker Hub 0.1.7).
  - Consumers pin by digest.
- **CI.** golangci-lint (pinned), vet, race, govulncheck (pinned), Trivy fs (vulns, misconfig,
  secrets), SHA-pinned actions, least privilege, weekly grouped Dependabot, and the same checks on the
  tagged commit plus an image scan before signing.

### F9 — The image is pushed to the registries before it is scanned and signed

**Severity:** Info   **Disposition:** ADJUDICATED
**Where:** `.github/workflows/docker-publish.yml`, `publish-public`: "Build and push" → "Scan the
published image" → "Sign images (keyless)".

**Issue:** the IMG lens asks for the image to be scanned before it is signed or pushed. The Trivy image
scan needs a registry reference, so the image (and its tags, including `latest` for stable tags) is
published first; a failing scan stops the job before cosign, leaving a pushed, unsigned image.

**Impact:** a tag could briefly point at an unsigned image with a fixable CRITICAL/HIGH finding. Every
consumer pins by digest and the workspace verifies the cosign signature (`verify-digests.sh`), so an
unsigned digest cannot be deployed through the normal flow.

**Remediation / evidence:** reviewed and accepted as intended: scan-before-sign holds (the lens's "before
it is signed or pushed" is met on the signing side); pushing first is how the scanner is given the
digest. A stricter order (build to a local store, scan, then push) would need the multi-arch push to be
split from the build and is not required while consumers pin by digest. Revisit if a consumer ever pulls
by tag.

### F10 — The Dockerfile is built and exercised only at release, not in PR CI

**Severity:** Info   **Disposition:** ACCEPTED-RISK
**Where:** `.github/workflows/go-ci.yml` (no `docker build` job); `.github/dependabot.yml` (docker
ecosystem, weekly, grouped); IMG lens §C.

**Issue:** CI lints, tests, vuln-scans and Trivy-scans the source and the Dockerfile's configuration,
but no job builds the image or runs the container (health endpoint, served headers, non-root) before a
tag. A Dependabot bump of the builder digest is therefore first built at release time.

**Impact:** a bad builder bump or a Dockerfile regression shows up when the release is attempted.
It fails closed: the toolchain check in the Dockerfile and the build step run before any push, so no
bad image is published.

**Remediation / evidence:** accepted because release is fail-closed and each consumer image is built
`FROM` this one and run in the cluster (the status page and the e2e runs exercise it). The exact fix is
S2: a PR-time `docker build` and a container smoke test (nonroot, `/healthz`, headers, a 404) in
`go-ci.yml`, which `verify` would then also run on the tag. Pre-v0.2 policy lets it wait.

### F11 — Shipped Go stdlib carried eleven net/http and net/textproto advisories (found 2026-10-09)

**Severity:** Low   **Disposition:** RESOLVED (0.1.7, `e1a71c5`)
**Where:** `go.mod` (`toolchain`), `Dockerfile:4` (builder tag and digest).

**Issue:** govulncheck reported eleven advisories (GO-2026-6607 to GO-2026-6617) against `net/http` and
`net/textproto` in Go 1.26.7, the toolchain of 0.1.6. The advisory texts were not re-read for this
audit; govulncheck lists only symbols the binary reaches, and this server is a `net/http` server.

**Impact:** an internet-facing server for every Meddleware front end, behind Cloudflare, ran a stdlib
with known advisories until the fix.

**Remediation / evidence:** 0.1.7 (`e1a71c5`, "Go 1.26.9 (stdlib advisories GO-2026-6607…6617)") sets
`toolchain go1.26.9` and moves the builder to `golang:1.26.9-bookworm@sha256:d9c68c2c…`; the Dockerfile
check keeps the two equal. The Publish run for `v0.1.7` passed govulncheck and the image scan, and the 0.1.7
digest `sha256:2e227311…` is pinned by all eleven consumers and `config/images.yaml`. This was caught by the
F2 gate (govulncheck on the toolchain that is shipped), which the first pass could not run (403).

---

## Section A — Invariant verification matrix

| # | Invariant (source) | Enforced at | Proven by | Status |
| --- | --- | --- | --- | --- |
| A1 | Cascade filesystem; never flattened (CLAUDE.md) | `cascadeFS` (`main.go:453`) | cascade tests | HOLDS |
| A2 | Directory listing disabled; `safeDir` outermost (CLAUDE.md) | `safeDir` (`:524`), `fileHandler` (`:254`) | listing tests | HOLDS |
| A3 | Served paths confined to the serve root (GO-M3; CLAUDE.md invariant 2) | `rootedFS` / `os.OpenRoot` (`:473`), `noDotfiles` (`:500`) | `TestSymlinkEscapeRefused`, `TestDotfilesHidden`, `TestRootedFSMissingDirServesNothing`, traversal and encoded-traversal tests | HOLDS (F1 RESOLVED) |
| A4 | Security headers on every route; never bypassed (CLAUDE.md) | `secureHeaders` before mux (`:551`) | header tests | HOLDS (CSP default off — F4, ACCEPTED-RISK) |
| A5 | GET/HEAD only | `methodGuard` (`:427`) | method tests | HOLDS |
| A6 | Immutable caching never applied to errors or navigations | `setCacheControl` + stdlib `serveError` + `no-cache` overrides | 2026-10-03 probe; no regression test | HOLDS (code-only; stdlib-dependent — S1) |
| A7 | Server timeouts; graceful shutdown (GO-M1, GO-M8) | `http.Server` (`:173`); signal handler | inspection | HOLDS (30-second write cap — F7) |
| A8 | CI gates on the released commit (GO-M7) | `docker-publish.yml` `verify` → `go-ci.yml` | Publish run 37964221333; the unpublished `0.1.5` | HOLDS (F2 RESOLVED) |
| A9 | Signatures verifiable against a pinned identity (IMG-M7) | release summary; SECURITY.md | inspection; `verify-digests.sh` 16/16 | HOLDS (F3 RESOLVED) |
| A10 | Docs discipline (CLAUDE.md) | — | grep | GAP — F6 (DEFERRED, pre-mainnet) |
| A11 | The binary is built with the toolchain CI checked (IMG scan-before-sign) | `go.mod` `toolchain` + Dockerfile `grep` | build fails on mismatch; Publish run | HOLDS (F2, F11 RESOLVED) |
| A12 | Pushed image scanned before it is signed | "Scan the published image" step before "Sign images" | Publish run steps in order | HOLDS (F9 ADJUDICATED: push precedes scan) |

---

## Section B — Supply-chain, publish-authority & capability matrix

### B.1 Dependency & CVE risk

| Component | Version | Status |
| --- | --- | --- |
| Go stdlib | go.mod `toolchain go1.26.9`; CI (setup-go) and image both 1.26.9 | govulncheck v1.8.0 green in the `v0.1.7` Publish run; not run locally (F11) |
| Go modules | none (`go.mod` has no `require`; no `go.sum`) | nothing to scan |
| `golang:1.26.9-bookworm` (builder) | `sha256:d9c68c2c…` | builder only; nothing copied except the binary and rendered `public/` |
| Runtime | `scratch` | no OS packages |
| Image scanner | Trivy (`aquasecurity/trivy-action` v0.36.0): filesystem scan in CI; image scan in release | green in the `v0.1.7` Publish run (CRITICAL/HIGH, `ignore-unfixed`) |
| Dependabot | `.github/dependabot.yml`: docker and github-actions, weekly, grouped | no `gomod` entry needed (no modules); builder bumps are checked by the Dockerfile toolchain assertion (F10) |
| First-party runtime image | this is the first-party runtime image (consumers use `0.1.7@sha256:2e227311…`) | all eleven consumers on `0.1.7` |

### B.2 Publish authority & CI

| Authority | Where | Custody | Gates |
| --- | --- | --- | --- |
| Push to quay.io / Docker Hub | `docker-publish.yml` (tag `v*`) | repo secrets (`QUAY_*`, `DOCKERHUB_*`) — inventory is the maintainer item "Image registry credentials" in `OPERATOR_TASKS.md` | `verify` = full Go CI; image scan (F2) |
| Signing / attestation | same | OIDC keyless (`id-token: write` on the publish job only) | — |
| Self-hosted registry | same | secrets | `continue-on-error` (best-effort, unsigned); its login fails without credentials (maintainer item, same `OPERATOR_TASKS.md` row) |

#### CI & release integrity

| Item | Holds? | Evidence |
| --- | --- | --- |
| Actions pinned to SHAs | Yes | both workflows (every `uses:` has a commit SHA) |
| Least privilege | Yes | `contents: read` at workflow level; write scopes on the publish job only |
| Tag-gated publish | Yes | `on.push.tags: v*` |
| Idempotent re-run | Partly | no guard: a re-run rebuilds and re-pushes the same tags (a new digest) and signs it. Consumers pin by digest and a version is never reused in practice (`0.1.5` was skipped, not re-tagged) — acceptable pre-v0.2 |
| Release gate equals CI | Yes | `verify` calls `go-ci.yml` (F2) |
| Lint, govulncheck, Trivy | Yes in CI and at tag | F2 |
| Image scan before signing | Yes | F2, F9 |
| Signature + SBOM + provenance | Yes | Docker Hub referrers (3 Sigstore bundles) |
| Verification guidance pins the signer | Yes | F3 |
| Automated dependency updates | Yes | `dependabot.yml` |
| Secrets never echoed | Yes | no `set -x`; secrets only in `with:`/`env:` of login steps |
| Real funds / test-only modes | N/A | no chain actions; no test mode |
| OIDC trusted publishing | N/A | container registries use tokens (inventory: maintainer item above) |

### B.IMG-1 Publish & attestation

| Registry | Signature | SBOM | Provenance | Notes |
| --- | --- | --- | --- | --- |
| Docker Hub | ✓ (referrer) | ✓ SPDX attestation (referrer) | ✓ BuildKit + GitHub | three Sigstore bundle referrers on `sha256:2e227311…` (2026-10-09) |
| quay.io | per workflow | per workflow | per workflow | the Publish run succeeded for all steps; not queried from this sandbox |
| Self-hosted | ✗ | ✗ | BuildKit | best-effort mirror; fails without credentials |

---

## Section C — Test-coverage & hermetic/live split

### C.1 Coverage grade — A− (37 tests, `-race`, 91.9%; 2026-10-09)

| Dimension | Assessment |
| --- | --- |
| Happy path | Cascade, SPA, clean URLs, 404 page, immutable prefix, precompressed, headers, nonce, healthz, version |
| Error path | 405; missing files; listing refusal; traversal (plain and encoded); symlink escape; dot-files; unopenable root |
| Boundary | **Missing:** `q=0` and `Vary` (F5); a 404 under the immutable prefix (S1); `script-src-elem` (F4) |
| Security-relevant | Strong for path syntax, path confinement and headers |

### C.2 Hermetic vs. live paths

| Path | Hermetic? | Deferred to | Tracking |
| --- | --- | --- | --- |
| The image as built (nonroot, healthcheck, served headers) | no | the Publish build, the consumers' images in the cluster; no container smoke test in CI | F10, S2 |
| Cloudflare caching behaviour | no | live edge | A6 |
| Signature verification against the registry | no | `verify-digests.sh` (workspace), run 2026-10-09 | F3 |

---

## Section D — Deployment-readiness gates

### pre-localnet

- [x] stdlib only; listing disabled; GET/HEAD; headers on every response — F8
- [x] digest-pinned builder; `scratch`; nonroot; no secrets in args — F8
- [x] symlink and dotfile confinement — F1 RESOLVED (0.1.6, `192b01f`; `TestSymlinkEscapeRefused`, `TestDotfilesHidden`)

### pre-testnet *(consumers are live)*

- [x] lint, govulncheck and Trivy at tag time; image scan; toolchain aligned — F2 RESOLVED (Publish run for `v0.1.7`)
- [x] pinned-identity verification guidance — F3 RESOLVED (release summary, SECURITY.md)
- [x] stdlib advisories cleared — F11 RESOLVED (Go 1.26.9)
- [x] `SECURITY.md` present with invariants — 0.1.6

### pre-mainnet

- [ ] docs discipline restored — F6 (DEFERRED; docs-only)
- [ ] regression test pinning "no caching headers on errors" (S1) and a container smoke test (S2, F10) — suggestions, not findings; A6 HOLDS code-only meanwhile
- [ ] CSP default and nonce handling decided — F4 / OQ2 (maintainer decision)
- [ ] external review — maintainer item (`OPERATOR_TASKS.md` "Funding, grants and an external audit")

---

## Cross-project themes

- **One runtime, every front end.** Header and cache semantics here are the VUE-M8 baseline for all
  apps. F1 was fixed once for every host (all eleven consumers moved to `0.1.7`); F4 stays a
  configuration choice that every consumer makes (CSP in each Dockerfile).
- **Supply chain & release integrity.** No modules, so no lockfile to carry; builder pinned by digest and
  checked against the `go.mod` toolchain; actions pinned by SHA; the release runs full CI and an image scan
  before signing; Dependabot (docker, github-actions) is committed. CVE status: govulncheck green on 1.26.9
  at the `v0.1.7` release; publish authority is the registry token inventory (maintainer item).
- **Signature verification is only as good as the identity.** This repository now prints and documents an
  anchored identity (F3). The same check applies to every image repository's release summary and docs
  (sui-indexer, platform-probe, registry services; their audits) and to the cluster admission policy, which
  today pins the org's workflows rather than one repository and runs in `warn` mode (OQ3; PLATFORM).
- **Toolchain drift between CI and image builders** (F2) is closed here by the Dockerfile assertion; it
  applies to every Go and Node image whose builder digest is pinned separately from the language version
  CI uses (the other three Go services adopted the same pinning in their 2026-10-09 releases).
- **Wire-format coupling and on-chain-truth boundary.** N/A: the server holds no chain logic, ABI or
  accounting; it only serves bytes and headers.
- **Chain-access layering and on-chain ID/ABI coupling.** N/A (no package or object IDs; it never calls a
  chain endpoint).
- **Deployment readiness.** Section D is current; the unticked items are docs, a maintainer decision
  (OQ2), suggestions and the external review.
- **Pre-v0.2 policy:** F4, F5 and F6 may change defaults without shims. The next release is 0.1.8;
  consumers move to a new base together (all eleven are on 0.1.7).

---

## Normative requirements (MUST / MUST NOT)

1. MUST serve only files inside the serve root, including through symlinks, and MUST NOT serve dot
   paths other than `/.well-known/` — **holds** (F1; `os.OpenRoot`, `noDotfiles`, three tests).
2. MUST run the same lint, vulnerability and secret checks on the released commit and image as on
   branches — **holds** (F2; `verify` reuses `go-ci.yml`, image scan before cosign).
3. MUST publish verification instructions that pin the signing workflow identity — **holds** (F3).
4. MUST build the shipped binary with the toolchain that govulncheck checks — **holds** (F2, F11;
   Dockerfile assertion).
5. MUST honour `Accept-Encoding` q-values and send `Vary` on every negotiable response before any
   consumer enables `PRECOMPRESSED` — **does not hold; latent** (F5, ACCEPTED-RISK).
6. MUST keep security headers on every response, GET/HEAD only, and directory listing off — **holds**
   (A2, A4, A5).

**GO lens baseline:**

| ID | Holds? | Evidence |
| --- | --- | --- |
| GO-M1 | yes (read 15 s covers headers, write 30 s, idle 60 s; GET/HEAD never read bodies; no `MaxHeaderBytes`, so the 1 MiB Go default applies) | A7 |
| GO-M2 | yes (the healthcheck has a 5-second timeout; no other outbound calls) | — |
| GO-M3 | yes | F1, A3 |
| GO-M4 | N/A | — |
| GO-M5 | yes (no secrets; logs exclude IPs and headers) | — |
| GO-M6 | N/A (no rate limits; per-IP limits are at the ingress) | — |
| GO-M7 | yes (CI and tag) | F2, A8 |
| GO-M8 | yes | A7 |

**IMG lens baseline:**

| ID | Holds? | Evidence |
| --- | --- | --- |
| IMG-M1 | yes (builder digest; `scratch`) | F8 |
| IMG-M2 | yes (`.dockerignore` covers `.env*`, tests, docs, `.github`, the Makefile; selective `COPY` of `go.mod`, `*.go` and `public/`) | — |
| IMG-M3 | yes (stdlib, no downloads; `CGO_ENABLED=0`; `sed` in the builder, no `apt-get`) | — |
| IMG-M4 | yes (no secrets in args or layers) | front matter |
| IMG-M5 | yes (image `USER 65534`; consumer pods enforce `runAsNonRoot`, read-only root, no escalation, dropped capabilities, `RuntimeDefault`, no service-account token) | front matter; pod specs |
| IMG-M6 | yes for consumers (digest pins from `config/images.yaml`) | deployment status |
| IMG-M7 | yes (signature, SPDX attestation, provenance; the verification identity is pinned) | F3, B.IMG-1 |
| IMG-M8 | yes (scan before sign; SBOM from the image; pinned identity; no redistributed third-party code, so no licence notices beyond 0BSD) | F2, F9 |

**VUE hosting rows (the runtime's side of B.VUE-1 / VUE-M8):** `X-Content-Type-Options: nosniff`,
`X-Frame-Options: SAMEORIGIN`, `Referrer-Policy` always; HSTS and `Permissions-Policy` by default (an
empty variable omits them); CSP only when configured, with `script-src` nonce handling. The effective CSP
per consumer is that consumer's audit; the image's own default page carries none (F4).

## Implementation suggestions (SHOULD / MAY)

- **S1** SHOULD add a test asserting that a missing file under `CACHE_IMMUTABLE_PREFIX` returns 404
  **without** `Cache-Control`, so a Go upgrade or `GODEBUG=httpservecontentkeepheaders=1` cannot
  silently reintroduce year-long caching of 404s across every app.
- **S2** SHOULD add a PR-time `docker build` and a container smoke test (run the image; check nonroot,
  `/healthz`, the headers and a 404) in `go-ci.yml` so `verify` also covers it (F10).
- **S3** MAY generate `public/llms.txt` and the default page's option table from one source (the
  package comment or a table file) to stop F6 recurring.
- **S4** MAY repeat the identity-pinned `cosign verify` command in the README and, when the cluster
  policy moves to enforce, pin it per repository (OQ3).

## Open questions (`OQ#`)

1. **OQ1** F1: are any consumer deployments mounting volumes into `SERVE_DIR` (ConfigMaps), and is
   `automountServiceAccountToken` disabled for these pods? (2026-10-09 evidence: `automountServiceAccountToken:
   false` is set on every consumer pod; no consumer deployment mounts a volume into
   `/app/public`: the only `mountPath` in the ten consumer manifests is the status deployment's
   `/etc/platform-probe`. The question stays open for the maintainer to confirm no out-of-tree deployment does.)
2. **OQ2** F4: should the image ship a strict default CSP, which consumers override, rather
   than none?
3. **OQ3** F3: does the cluster admission policy (if any) verify static-server-based images against
   a pinned identity? (2026-10-09 evidence: a `ClusterImagePolicy` exists in `warn` mode and pins the GitHub
   issuer plus any `meddleware-org` workflow; whether to enforce it, and per repository, is a platform decision.)

## Risks

- **Blast radius:** a defect here (headers, caching, confinement) reaches every Meddleware host at
  once. All eleven consumers are on `0.1.7`, but every fix still needs a coordinated base bump.
- **Implicit stdlib guarantees:** the cache safety of 404s rests on Go's error-path behaviour (S1).
- **Stdlib currency:** a new Go advisory affects every host until the next release; the release gate
  catches it only when CI runs or a tag is pushed, and the `go.mod` toolchain line is not covered by a
  Dependabot ecosystem (no `gomod` entry), so a Go patch release is picked up by hand.
- **Registry custody:** a leaked registry token could publish an unsigned image under an existing tag;
  digest pinning plus signature verification bound it, but the token inventory is a maintainer item.
- **Admission is warn-only:** the cluster does not yet refuse an unsigned or wrongly signed image (OQ3).

---

## Re-verification log

- 2026-10-03 — first-pass baseline at `f596636` (release `v0.1.4` = `448dc45`; Docker Hub 0.1.4
  `sha256:14668bc2…`).
  - **Lenses:** AUDIT_TEMPLATE.md (2026-10-02) + GO (2026-09-30) + IMG (2026-09-30), with VUE-M8
    assessed as an enabled property.
  - **Measured:** vet clean; 34 tests race-clean; coverage 90.9%.
  - **Probes:** a scratch probe (deleted afterwards) **refuted** the immutable-404 hypothesis
    (stdlib strips caching headers; source confirmed in `fs.go` `serveError`) and confirmed F1, F4
    and F5.
  - **Registry API:** confirmed the attestations and the 1.26.7 stdlib.
  - **Not run:** govulncheck (403), golangci-lint (not installed), quay.io, the cosign identity
    check.
  - **Recorded:** F1–F8; OQ1–OQ3.
  - **No findings resolved:** by maintainer instruction this pass only recorded findings.
- 2026-10-09 — re-verified at `main` = `v0.1.7` = `e1a71c5` (Docker Hub `0.1.7`
  `sha256:2e227311…2379`). Read-only; no code changed.
  - **Resolved:** F1 (0.1.6 `192b01f`: `os.OpenRoot`, `noDotfiles`; three tests), F2 (reusable CI as the
    release gate, image scan before cosign, `toolchain` directive and Dockerfile check; the unpublished
    `0.1.5` shows the gate working), F3 (anchored identity in the release summary and SECURITY.md).
  - **New:** F11 RESOLVED (Go 1.26.7 carried eleven stdlib advisories; fixed in 0.1.7 `e1a71c5`); F9
    ADJUDICATED (image pushed before scan, signed after); F10 ACCEPTED-RISK (no image build in PR CI).
  - **Re-checked, unchanged in code:** F4, F5, F7 now ACCEPTED-RISK with reasons (all eleven consumers set a
    CSP and none enables `PRECOMPRESSED`); F6 DEFERRED to the pre-mainnet gate (the five-option table is
    unchanged; SECURITY.md invariants item fixed in 0.1.6; two more comment slips added).
  - **Stale facts corrected:** Go 1.26.9 / builder `golang:1.26.9-bookworm@sha256:d9c68c2c…` (was 1.26.6/1.26.7
    and `6ef6e30f…`); consumers all on `0.1.7@sha256:2e227311…` (was 0.1.3/0.1.4); 37 tests, 91.9% (was 34,
    90.9%); `main.go` 662 lines, `main_test.go` 962.
  - **Lenses:** base, GO, IMG and VUE (hosting rows) all dated 2026-10-08, matching the registry; Section A
    gained A11–A12, B gained a Dependabot and CI-integrity table, the GO/IMG baselines were re-assessed
    (IMG-M8, new in the IMG lens, holds).
  - **Gates:** pre-localnet and pre-testnet ticked with evidence; pre-mainnet keeps F6, the S1/S2 suggestions,
    OQ2 and the external review.
  - **Evidence of run:** `go vet` clean; `go test -race -cover` ok (37 tests, 91.9%); GitHub run 37964221333
    (all verify jobs, scan, sign, SBOM, provenance success; the best-effort private-registry job fails at login);
    Docker Hub digest and referrers; `config/images.yaml`; consumer Dockerfiles and pod specs.
  - **Not verified:** govulncheck and golangci-lint locally (only through the Publish run); quay.io manifest
    directly; the advisory texts for GO-2026-6607…6617.
  - **Contradiction with the workspace summary:** the Dockerfile has `USER 65534`, not `USER 65534:65534`;
    the group is 65534 through the `/etc/passwd` entry and consumer pods set `runAsUser: 65534`, so no gap.
  - **OQ1–OQ3** stay open with the evidence now in the questions.

## Pre-save consistency checklist (this pass)

- [x] Section A ↔ findings: A3 HOLDS (F1), A8 HOLDS (F2), A9 HOLDS (F3), A10 GAP (F6), A11 and A12 (F2, F11, F9); A6 stdlib note (S1).
- [x] Finding header ↔ body: consistent (F6's struck-through line records the fixed SECURITY.md item).
- [x] Template line: base + GO + IMG + VUE (hosting rows) with the registry dates; untriggered lenses named.
- [x] Closing four-part structure present.
- [x] Open questions stay open.
- [x] Section D ↔ dispositions.
- [x] Executive summary ↔ dispositions (4 RESOLVED, 4 ACCEPTED-RISK, 1 ADJUDICATED, 1 DEFERRED, 1 Positive) and ceiling (High; realised Low at the first pass, Info now).
- [x] C.1 counts measured 2026-10-09.
- [x] Re-verification log entry added.
