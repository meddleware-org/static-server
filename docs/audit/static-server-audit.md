# Security Audit — `static-server`

**Classification:** Internal security review (re-verified 2026-10-09; fix wave 2026-10-18, released and deployed as 0.1.8 on 2026-10-18; F4 resolved in 0.1.9, released 2026-10-19 (image published and signed; not deployed directly, consumers still run their 0.1.8-based images) — awaiting external review)
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

- **2026-10-22: `v0.1.11` = `2ed0ebf` is the current release** (Go 1.26.10, F13). `v0.1.10` (2026-10-20, native cross-compile, no QEMU) precedes it. The eleven consumer images were rebuilt on 0.1.11 on 2026-10-22 (their own audits); the image is still not deployed directly.
- Release `v0.1.7` = `e1a71c5` (2026-10-09) = HEAD of `main`; Publish run 37964221333 succeeded (full CI
  through the reusable workflow, Trivy image scan, cosign, SBOM and provenance attestations). Tags
  `v0.1.5` (never published: the release gate caught a lint finding; Docker Hub has no `0.1.5`) and
  `v0.1.6` (`86e1e70`) precede it.
- Docker Hub `meddleware/static-server:0.1.7` = index `sha256:2e227311…2379` (re-read from the registry
  2026-10-09; it was then the digest pinned in `config/images.yaml`, until 0.1.8 on 2026-10-18). Its referrers carry three Sigstore
  bundles (cosign signature, SBOM attestation, build provenance). `0.1.6` = `sha256:be51c4ee…`. quay.io
  was not queried from this sandbox; the Publish run pushed and scanned the quay.io image.
- **Consumers (all eleven were pinned by digest to `0.1.7@sha256:2e227311…` from 2026-10-09 and to `0.1.8@sha256:e32cf42b…` from 2026-10-18; on `main` all eleven now pin `0.1.9@sha256:af6cc284…`, read from each repo's `Dockerfile` on GitHub 2026-10-19 — merged Dependabot docker PRs, not re-released or redeployed, so the running pods are still the 0.1.8-based images):** access-gate-ui, dao-ui (retired
  from the cluster, still published), dashboard, dev, docs, landing, sealed-storage-ui, status-ui,
  token-issuance-ui, treasury-ui, walrus-storage-ui. Every consumer Dockerfile sets `CONTENT_SECURITY_POLICY`
  and `CACHE_IMMUTABLE_PREFIX`; the SPAs set `SPA_FALLBACK=true`. Deployed images were cosign-verified
  2026-10-09 and again 2026-10-18 (`bootstrap/images/verify-digests.sh`, 16/16 valid both times). None sets `PRECOMPRESSED`.
- **Release `v0.1.8` (2026-10-18) = `3f1ddea`** (fix-wave commit `ed7c737` plus the audit commits), digest
  `0.1.8@sha256:e32cf42b182d270c4ef57181e5588e7df454a42cac66e83d8ae59d9c7b7c4000` (quay.io and Docker Hub, same
  digest; pinned in `config/images.yaml`). The cosign keyless signature verified 2026-10-18 against the workflow-anchored identity (`bootstrap/images/verify-digests.sh`, 16/16 valid). Rolled out to the cluster on 2026-10-18 (the consumers' rebuilt
  images were applied with `kubectl apply -k` / `apply.sh`); every rollout succeeded. The Publish run for the tag (39115965481) was green, including the new CI `image` (build and smoke test) job. All eleven consumers' Dockerfiles
  now `FROM` `0.1.8@sha256:e32cf42b…` (read from the checkouts 2026-10-18), and all eleven public pages answered 200 on
  2026-10-18 with CSP, HSTS, `nosniff`, `X-Frame-Options`, `Referrer-Policy` and `Permissions-Policy` (curl -I; the 11
  pages also loaded in headless Chromium with no console errors or CSP violations, per the release facts). Supersedes the
  0.1.7 line above, which was the deployed base from 2026-10-09 to 2026-10-18.
- **Release `v0.1.9` (2026-10-19) = `30676d2`** (code commit `e2a9aed` plus the audit-hash commit; Publish run 39181842677
  succeeded: Image (build and smoke test), Test, govulncheck, Trivy filesystem scan, Lint and the quay.io + Docker Hub publish
  jobs; the best-effort self-hosted-registry job failed at login as before). The image bakes the restrictive default CSP (F4,
  OQ2 option (c)). Digest `0.1.9@sha256:af6cc284dace0d7bcacca61765f1e5efa0681da503c607f6e13ea35ce19edc72`: the quay.io
  manifest list (4 child manifests present, quay API 2026-10-19) and the Docker Hub tag report the same digest; it is pinned in
  `config/images.yaml`. The cosign signature was reported verified against `docker-publish.yml@refs/tags/v` by the release
  orchestrator; this pass did not re-run cosign. **The image is not deployed anywhere directly**; the eleven consumer images
  `FROM` it by digest. Because each sets its own `CONTENT_SECURITY_POLICY`, rebuilding them on 0.1.9 changes nothing for them,
  so they have not been re-released. `main` is one commit past the tag (`b566d19`, `.github/dependabot.yml` ignore rules only).
- Consumer pods set `automountServiceAccountToken: false` (every `post-bootstrap/*/base` deployment and
  the `k8s/base` status and registry pods).

**Review date:** 2026-10-03; re-verified 2026-10-09; fixes 2026-10-18; image build 2026-10-20 (0.1.10)
**Reviewer:** Internal review
**Severity ceiling:** High.

- It serves every public front end, and its headers are those front ends' browser security policy
  (CSP, HSTS, framing).
- A path-confinement or header defect would affect every app at once.
- Realised ceiling: **Low** at the first pass (F1–F3) and F12 (2026-10-18), all four since RESOLVED; what
  remains open is Info.

**Status:** re-verified 2026-10-09 (first pass 2026-10-03); fix wave 2026-10-18 (F5, F6, F10, F12 RESOLVED in 0.1.8; released and deployed 2026-10-18, signature verified, rollout succeeded); OQ2 decided and F4 RESOLVED in 0.1.9 (released 2026-10-19, tag `v0.1.9`; the image is published and signed, not deployed directly). The CHANGELOG records an earlier internal
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
| Images | `quay.io/meddleware-org/static-server:0.1.9@sha256:af6cc284…edc72`; `docker.io/meddleware/static-server:0.1.9@sha256:af6cc284…edc72` (released 2026-10-19, digests read from both registries; 0.1.8 was `sha256:e32cf42b…4000`, released and signature-verified 2026-10-18; 0.1.7 was `sha256:2e227311…2379`); `registry.meddleware.co.uk/…` (best-effort, fails without credentials — maintainer item) |
| Base images | builder `golang:1.26.9-bookworm@sha256:d9c68c2c…`; runtime `scratch` |
| Runtime user | `USER 65534` in the Dockerfile (`nobody`, 65534:65534 via the generated `/etc/passwd`); consumer pods add `runAsNonRoot`, `runAsUser: 65534`, no privilege escalation, all capabilities dropped and `RuntimeDefault` seccomp (checked in the ten consumer deployments that run it: access-gate-ui, dashboard, dev, docs, landing, sealed-storage-ui, token-issuance-ui, treasury-ui, walrus-storage-ui, status) |
| Runtime FS | `/app/default` (baked), `/app/public` (operator mount or consumer `COPY`); consumer pods set `readOnlyRootFilesystem: true` |
| Deployed by | consumers' images `FROM` this one by digest; digest source `config/images.yaml` |
| Build args | `VERSION`, `VENDOR`, `DESCRIPTION`, `SOURCE_URL`, `DOCUMENTATION_URL`, `IMAGE_URL`, `SITE_*`, `OG_URL`, `IMAGE_REF`, `SCHEMA_LICENSE_URL` — labels and default-page text; no secrets |

**Location:** `static-server/docs/audit/static-server-audit.md` (canonical: the repo-local audit). The
`docs/` directory is not yet committed in the repo.

> **Access note (2026-10-09):** read-only re-verification of `workspace/repos/static-server` at `main`
> (`e1a71c5`). No code, commit or push was made by this pass.
>
> **Access note (2026-10-18):** the fix wave edited this repository locally (0.1.8, commit `ed7c737`); at
> the time of that pass there was no tag, push or publish. Later the same day `v0.1.8` was tagged and
> released (see Deployment status).

---

## Executive summary

`main.go` (711 lines) holds all the logic. There are 47 tests (`main_test.go`, 1334 lines) over real
HTTP (`httptest`). Since the 2026-10-03 pass the three Low findings were fixed and released (0.1.6 and
0.1.7), the Go toolchain advisory found on 2026-10-09 was fixed (0.1.7), and the remaining Info items
were re-checked and given reasoned dispositions. The 2026-10-18 fix wave (0.1.8, commit `ed7c737`) closed
F5, F6 and F10, found and fixed F12 (the 405 response lacked the security headers), and fixed the
nonce and `Cache-Control` edge cases of F4 and F7. A second pass on 2026-10-19 (0.1.9, released the same day) took the
maintainer's decision on OQ2, option (c): the image bakes in a restrictive same-origin default CSP (F4 RESOLVED).

**What holds (verified):**

- **Stdlib only.** No third-party modules.
- **Paths stay in the serve root (F1, RESOLVED).** `rootedFS` opens each root with `os.OpenRoot`, so a
  symlink that leaves it is a 404; `noDotfiles` hides dot-paths except a leading `/.well-known/`.
- **Directory listing disabled.** `safeDir` wraps the whole cascade, and a directory without
  `index.html` returns 404.
- **Read-only.** GET and HEAD only (405 with `Allow` otherwise).
- **Headers on every response, including errors** (true for the 405 only since 0.1.8, F12).
  - Always: `nosniff`, `X-Frame-Options: SAMEORIGIN`, Referrer-Policy.
  - When configured: CSP with a per-response 128-bit nonce, HSTS (default one year,
    `includeSubDomains`), Permissions-Policy.
- **Traversal confined.** Encoded traversal never leaves the serve root (tested).
- **No immutable 404s.**
  - The 2026-10-03 probe refuted the hypothesis that a missing fingerprinted asset is served `404`
    with `Cache-Control: public, max-age=31536000, immutable`: since Go 1.23, `http.FileServer`'s
    error path deletes `Cache-Control`, `Content-Encoding`, `Etag` and `Last-Modified`.
  - The safety still depends on the toolchain and `GODEBUG` defaults; since 0.1.8
    `TestNoCachingHeadersOnErrors` pins it (S1), and it fails under
    `GODEBUG=httpservecontentkeepheaders=1`.
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
  - Multi-arch, built natively: the builder cross-compiles on the build host and the release has no QEMU step (0.1.10).
  - Keyless cosign signature, SPDX SBOM attestation and GitHub build-provenance attestation (three
    Sigstore bundle referrers on the Docker Hub 0.1.7 index).
- **CI.** golangci-lint (pinned v2.13.0), `go vet`, race tests with coverage, govulncheck (pinned
  v1.8.0), a Trivy filesystem scan (vulns, misconfig, secrets), SHA-pinned actions, and Dependabot
  (weekly, grouped: docker and github-actions).
- **Measured (2026-10-19).** `go vet` clean; **47 tests pass with `-race`**; **coverage 93.0%**
  (Go 1.26.9, the `go.mod` toolchain); golangci-lint v2.13.0 0 issues; govulncheck v1.8.0 no
  vulnerabilities; `scripts/smoke.sh` passed against a rootless-Podman build of the image.

**Findings and dispositions (F1–F13):** 10 RESOLVED (F1, F2, F3, F4, F5, F6, F10, F11, F12, F13), 1 ACCEPTED-RISK
(F7), 1 ADJUDICATED (F9), 1 Positive (F8). None above Info is unresolved.

- **F4 — CSP was off by default.** Info; RESOLVED in 0.1.9 (the nonce edge cases in 0.1.8). The maintainer
  decided OQ2 for option (c): the image bakes a restrictive same-origin default CSP into a Dockerfile `ENV`;
  the Go default stays empty and all eleven consumers set their own, so none is affected. The cost lands on
  out-of-tree operators whose mounted content needs inline scripts, inline styles or CDN assets (see OQ2).
- **F5 — precompressed negotiation ignored `q=0`.** RESOLVED in 0.1.8 (q-values parsed, `Vary` on every
  asset response).
- **F6 — docs-discipline drift.** RESOLVED in 0.1.8 (every option in every reference surface; a test keeps
  it so).
- **F7 — minor runtime notes.** ACCEPTED-RISK; the SPA-fallback 404 header edge case was fixed in 0.1.8,
  the rest stays accepted.
- **F10 — no image build in PR CI.** RESOLVED in 0.1.8 (`image` job: build and smoke test).
- **F12 — the 405 response lacked the security headers.** Found and RESOLVED in 0.1.8.

**Posture:**

- A small, careful server with strong CI and release attestations; the path-confinement, release-gate
  and verification-guidance gaps found on 2026-10-03 are closed in code and released.
- The rare cache-header trap is closed by the Go stdlib and, since 0.1.8, pinned by a test (S1).
- What remains is the external review (pre-mainnet). The default-CSP decision is taken (OQ2, 0.1.9; the cost for out-of-tree operators is stated there).

---

## Threat model / trust boundaries

| Actor / source | Controls | Can do | Bounded by |
| --- | --- | --- | --- |
| Anonymous client (through Cloudflare and ingress) | method, path, query, headers (`Accept-Encoding`, `If-None-Match`, `Range`) | Request any path; probe for files | GET/HEAD only; `os.OpenRoot` confinement and `noDotfiles` (F1, RESOLVED); `safeDir`; server timeouts. `q=0` honoured since 0.1.8 (F5, RESOLVED) |
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
| Registry | the manifest for a tag | consumers pin by digest (all eleven verified 2026-10-09 and 2026-10-18); `verify-digests.sh` cosign-checks each pinned digest |

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
| `go vet ./...`, `go test -race -cover ./...`, gofmt (2026-10-19, Go 1.26.9) | clean; **ok** (47 tests), **coverage 93.0%** (0.1.9 tree) |
| govulncheck / golangci-lint | 2026-10-18: govulncheck v1.8.0 (Go 1.26.9) no vulnerabilities; golangci-lint v2.13.0 (the CI pin) 0 issues. 2026-10-09: not run locally; both ran green in the Publish run for `v0.1.7` (run 37964221333) |
| `scripts/smoke.sh` (2026-10-18, rootless Podman 4.9.3, `podman build --build-arg VERSION=smoke`) | passed; the image runs as `65534`, `--read-only --cap-drop ALL`; the in-image `-healthcheck` works; no Docker daemon was used (root-only socket) |
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
  The README does not repeat it. The workspace's `bootstrap/images/verify-digests.sh` verified each
  pinned digest against a per-image `https://github.com/meddleware-org/<name>/` identity prefix on
  2026-10-09 (16/16 valid); since 2026-10-18 its default identity is workflow-anchored (`^https://github\.com/meddleware-org/${name}/\.github/workflows/docker-publish\.yml@refs/tags/v` , 16/16 valid).
- **Cluster admission (OQ3):** the `ClusterImagePolicy` in `k8s/base/infrastructure/policy-controller`
  pins the GitHub issuer and a `meddleware-org/<any repo>/.github/workflows/…` regexp (any workflow of
  any org repository, not this one) and is in `warn` mode. That is a platform item (not this repository);
  see the PLATFORM audit.

### F4 — CSP was off by default, and the nonce handling had edge cases

**Severity:** Info   **Disposition:** RESOLVED (0.1.9, commit `e2a9aed`: restrictive default CSP in the image, OQ2 option (c); the nonce edge cases fixed in 0.1.8)
**Where:** `main.go:115-116` (`CONTENT_SECURITY_POLICY` default `""`; `CSP_NONCE` default `true`),
`secureHeaders` (`:585`), `withScriptNonce` (`:625`).

**Issue / Impact:**

- **Off by default.** No CSP is sent unless the operator sets one, including on the image's own
  default page. Every current consumer sets one (verified), but VUE-M8's "every hosting path"
  depends on configuration, not the runtime.
- ~~**Only `script-src` gets the nonce.**~~ A policy using `script-src-elem` (which takes precedence for
  `<script>` elements) blocked edge-injected scripts (probe). Fixed in 0.1.8.
- **The nonce disables `'unsafe-inline'`.** Under CSP2+ a nonce in `script-src` makes browsers
  ignore `'unsafe-inline'` there. No consumer uses `'unsafe-inline'` in `script-src` today (they use
  `'self'` plus hashes), but a future one would see its inline scripts break silently.
- **Nonce on every response.** The nonce is generated for every response, including assets, and is
  reused if an edge caches an HTML response with its header. This matters only if there is an HTML
  injection point, and this server has none.

**Remediation / evidence:** 2026-10-18 (0.1.8, commit `ed7c737`): `withScriptNonce` now appends the nonce to
`script-src` and `script-src-elem` (first occurrence of each only; a directive that is exactly `'none'` is
left alone). Pinned by `TestWithScriptNonce` (script-src-elem, repeated directives, `'none'`). The
`'unsafe-inline'` interaction is documented in the package comment, README, `.env.example`, `llms.txt`,
`public/llms.txt` and `public/index.html`. A `text/html`-only nonce was not done: the server has no HTML
injection point, the edge reads the header on HTML responses, and a per-response writer wrapper would add
code for no stated risk. The default was then left to OQ2. The 2026-10-09 re-check follows, and the 2026-10-19
0.1.9 resolution after it.

- **Resolution (2026-10-19, 0.1.9, commit `e2a9aed`).** The maintainer decided OQ2 for option (c). The
  Dockerfile now has `ENV CONTENT_SECURITY_POLICY="default-src 'self'; script-src 'self'; style-src 'self';
  img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'self'; form-action 'self'"`.
  The Go default (`envOr("CONTENT_SECURITY_POLICY", "")`) is unchanged, so the bare binary sends no CSP and
  the eleven consumer images (each sets `ENV CONTENT_SECURITY_POLICY="${CSP}"` from a non-empty `ARG CSP`
  default; grep of `repos/*/Dockerfile` 2026-10-19) replace the default and are unaffected. `frame-ancestors
  'self'` is the audit's option (c) text and matches `X-Frame-Options: SAMEORIGIN` (`'none'` would contradict
  it). `form-action 'self'` is added to the option (c) text. The nonce is appended to `script-src` as before.
  An operator overrides the default with their own `CONTENT_SECURITY_POLICY`, or sends no CSP by setting it to
  the empty string (`envOr` treats empty as unset, so no code or new knob was needed). The built-in default
  page needs nothing the policy blocks: its only inline script is the JSON-LD data block (not executed),
  `script.js` and `style.css` are same-origin files with no third-party URL, and there is no inline style or
  handler. Pinned by `TestImageDefaultCSP` (restrictive directives, no `unsafe-*`, no external origin, the
  policy stated verbatim on every doc surface and in the CHANGELOG, nonce handling), `TestBinaryCSPDefaultIsUnset`,
  `TestDefaultPageFitsDefaultCSP` and `scripts/smoke.sh` (the stock image sends the exact default with a
  fresh nonce on 200, 404 and `/healthz`; an operator value replaces it; an empty value sends no CSP). The page
  was also loaded in headless Chrome against the built image: `script.js` ran (the canonical link was added)
  with no CSP violation logged.
- **Residual cost.** Out-of-tree operators who mount content with inline scripts, inline styles or CDN assets
  on the stock image get that content blocked after upgrading to 0.1.9; they set `CONTENT_SECURITY_POLICY`
  (their own policy, or empty). Nothing in this repository can know how many there are. The bare binary still
  has no default, by decision.


- **Why accepted.** Every consumer image sets `CONTENT_SECURITY_POLICY` in its Dockerfile (all eleven,
  grep of `repos/*/Dockerfile`); their `script-src` values are `'self'`, `'self'` plus `'wasm-unsafe-eval'`
  or `'self'` plus two `sha256-` hashes (VitePress, checked at build time). None uses `script-src-elem`
  or `'unsafe-inline'`, so the nonce edge cases cannot bite today. The nonce is a random header value
  that lets only markup carrying it run, and the server has no HTML injection point.
- **Open choice (closed 2026-10-19).** Whether the image should ship a strict default CSP was OQ2, left
  open on 2026-10-09 because it changes what every unknown operator's mounted content may load. The
  maintainer decided it on 2026-10-19 (option (c)); see the resolution above.
- **If a consumer changes.** The `script-src-elem` handling and the `'unsafe-inline'` note are now in
  place (0.1.8); a `text/html`-only nonce remains optional.

### F5 — Precompressed negotiation ignores `q=0`, and only one variant carries `Vary`

**Severity:** Info (latent: no consumer sets `PRECOMPRESSED`)   **Disposition:** RESOLVED (0.1.8, commit `ed7c737`)
**Where:** `main.go:395-457` (`tryPrecompressed`, `acceptsEncoding`).

**Issue / Impact (probe-verified):**

- `strings.Contains(ae, "br")` serves `br` to `Accept-Encoding: br;q=0, gzip`, a client that
  explicitly refused it. Substring matching would also match tokens containing `br` or `gzip`.
- `Vary: Accept-Encoding` is added only when a compressed sibling is served. The identity response
  for the same URL has no `Vary`, so a shared cache can store it unkeyed. RFC 9110 expects `Vary` on
  every variant.

**Remediation / evidence:** 0.1.8 (commit `ed7c737`) implements the stated fix before any consumer enables
the feature. `acceptsEncoding` splits `Accept-Encoding` into whole tokens, matches them case-insensitively
and refuses a coding whose q-value is `0` or unparseable (the wildcard is not honoured: only codings named
explicitly are served). `tryPrecompressed` adds `Vary: Accept-Encoding` to every GET/HEAD response for a path with
an extension, served from a sibling or not. Pinned by `TestAcceptsEncoding` (18 cases),
`TestPrecompressedNegotiation` (q=0 falls to gzip or identity; exactly one `Vary` value; no `Vary` on a
navigation route) and `TestPrecompressedOffNoVary`. Before: re-checked 2026-10-09, still
`strings.Contains(ae, enc.name)` with `Vary` only on the compressed branch; accepted then because no
consumer enabled it (no `PRECOMPRESSED` in any consumer Dockerfile or `post-bootstrap` base manifest).

### F6 — Docs-discipline drift (CLAUDE.md requires every option in all reference surfaces)

**Severity:** Info   **Disposition:** RESOLVED (0.1.8, commit `ed7c737`)
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

**Remediation / evidence:** 0.1.8 (commit `ed7c737`). The table above is the state found on 2026-10-09 (re-checked
by grep of the five option names in each surface). Fixed: the five options are now in `public/llms.txt`,
`public/index.html`, `llms.txt` and `.env.example` (and `AGENTS.md`'s variable table, which listed only three
of the twelve); the three comment slips are corrected (the package comment gives `FALLBACK_DIR` as `""` in the
binary and `/app/default` from the image; `go-ci.yml` names `docker-publish.yml`; `docker-publish.yml`
says the SBOM is an anchore cosign attestation, not BuildKit's). More stale text found and fixed in the same
sweep: README, AGENTS, `llms.txt` and the Makefile and Dockerfile comments named `ci.yml`/`publish.yml`, an
`envsubst`/`gettext-base` build step (the Dockerfile uses `sed`), Go 1.24, and a 15-test count; `newHandler(fs)`
was `newHandler(cfg, fs)`. S3 is met by a test instead of a generator: `TestDocsListEveryOption` reads
every `env*("NAME"` in `main.go` and fails if README, both `llms.txt`, `public/index.html`, `.env.example` or
`AGENTS.md` omits it (or CHANGELOG, bar `PORT`).

### F7 — Minor runtime notes

**Severity:** Info   **Disposition:** ACCEPTED-RISK (the `serveIndex` header edge case fixed in 0.1.8)

- **Write timeout.** `WriteTimeout: 30s` cuts any response not finished in 30 s. Behind Cloudflare
  this is fine, but direct or slow clients downloading large assets (the 11.6 MB ui legal PDF, wasm
  bundles) can be truncated.
- ~~**Edge-case header.**~~ `serveIndex`'s `http.NotFound` path (no `index.html` at all) kept any
  `Cache-Control: immutable` set earlier for paths under the prefix. That needed a broken
  deployment. Fixed in 0.1.8: `serveIndex` deletes `Cache-Control` before the 404 (`TestSPAWithoutIndexIsPlain404`,
  and `TestNoCachingHeadersOnErrors` for the other error paths).
- **SBOM version.** The SBOM lists the module version as `UNKNOWN` (no VCS info in the Docker build
  context). Pass `-ldflags -X` (already done for `main.version`) and set
  `-buildvcs=false`/annotations, or record the version in the SBOM.
- **Default-page build args.** They are substituted into HTML (including the JSON-LD block) by `sed`
  without HTML or JSON escaping. Operator-controlled and documented (`|`/`&` note), but a `</script>`
  or quote in a build arg would break the page.

**Remediation / evidence:** re-read 2026-10-18: only the edge-case header has a single clear fix (above);
the other three have no single fix (a longer or per-write deadline is a tuning choice; the SBOM module
version needs a build-metadata decision; escaping build args needs an escaping scheme for `sed`), so they
stay accepted. Re-checked 2026-10-09; the code and Dockerfile were unchanged on these points. Accepted
with reasons: all public traffic reaches the server through Cloudflare (which buffers
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
  before routing, so headers are present on 404 and health responses, and (since 0.1.8, F12) on the 405
  (the first passes recorded the 405 as covered; it was not).
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
  - Multi-arch, built natively (cross-compiled on the build host, no QEMU, 0.1.10); keyless cosign, SLSA provenance (BuildKit `mode=max` + `attest-build-provenance`) and
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

**Severity:** Info   **Disposition:** RESOLVED (0.1.8, commit `ed7c737`)
**Where:** `.github/workflows/go-ci.yml` (no `docker build` job at the time); `.github/dependabot.yml` (docker
ecosystem, weekly, grouped); IMG lens §C.

**Issue:** CI lints, tests, vuln-scans and Trivy-scans the source and the Dockerfile's configuration,
but no job builds the image or runs the container (health endpoint, served headers, non-root) before a
tag. A Dependabot bump of the builder digest is therefore first built at release time.

**Impact:** a bad builder bump or a Dockerfile regression shows up when the release is attempted.
It fails closed: the toolchain check in the Dockerfile and the build step run before any push, so no
bad image is published.

**Remediation / evidence:** 0.1.8 (commit `ed7c737`) implements S2. `go-ci.yml` gains an `image` job
(`docker build --build-arg VERSION=smoke`, then `bash scripts/smoke.sh static-server:ci`), modelled on
sui-indexer's job; `docker-publish.yml`'s `verify` reuses `go-ci.yml`, so the tag runs it too. The script
is dependency-free (bash, curl and the engine) and takes about a second after the build. It runs the image
the way a cluster does (`--read-only --cap-drop ALL --security-opt no-new-privileges`) and checks: the image
`Config.User` is `65534`; the in-image `-healthcheck`; `/healthz` and `/version` (`smoke`); the rendered
default page from `/app/default`; the five always/default security headers and a CSP with a fresh nonce on
a 200, a 404 and a 405; no `Cache-Control` on a 404 under `CACHE_IMMUTABLE_PREFIX`; `/.env` and `/.git/config`
404; `POST` 405 with `Allow`. It works with `CONTAINER_ENGINE=podman`; run locally 2026-10-18 with rootless
Podman 4.9.3 (built from the Dockerfile with the pinned builder digest) and passed. The job itself runs only
in GitHub Actions (Docker is unavailable here). The first run found F12. Before: accepted because release was
fail-closed and each consumer image is built `FROM` this one and run in the cluster.

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

### F12 — The 405 response carried no security headers (found 2026-10-18)

**Severity:** Low   **Disposition:** RESOLVED (0.1.8, commit `ed7c737`)
**Where:** `main.go:233` (`newHandler`), `methodGuard` (`:461`), `secureHeaders` (`:585`).

**Issue:** the handler chain was `logRequest → methodGuard → secureHeaders → mux`, so `methodGuard` answered
a POST, PUT or DELETE with `http.Error` before `secureHeaders` ran. The 405 had `nosniff` (added by
`http.Error`) and `Allow`, but no `X-Frame-Options`, `Referrer-Policy`, HSTS, `Permissions-Policy` or CSP.
The first passes (F8, Section A, the executive summary) recorded "headers on every response, including
404, 405" and CLAUDE.md requires `secureHeaders` never be bypassed; no test covered the 405 headers.
The new container smoke test found it on its first run.

**Impact:** a 405 is a body-less error page, so the browser-side effect is small (a 405 is not framed or
rendered as a document by normal flows), but it broke the stated invariant (A4) and a header scanner would
flag it. It affected every consumer.

**Remediation / evidence:** 0.1.8 (commit `ed7c737`): `newHandler` is now `logRequest → secureHeaders →
methodGuard → mux`. Pinned by `TestSecurityHeadersOnMethodNotAllowed` (POST, PUT, DELETE, PATCH, OPTIONS: all
five headers and a nonce'd CSP) and by the smoke test (POST / is checked for all six headers).

### F13 — Trivy flags the Go 1.26.9 standard library (CVE-2026-111943) in the published binary, though govulncheck finds it unreachable (found 2026-10-22)

**Severity:** Low   **Disposition:** RESOLVED (0.1.11, commit `2ed0ebf`, 2026-10-22)
**Where:** `go.mod` (`toolchain`), `Dockerfile` (builder `golang:<patch>-bookworm@sha256:…`), `docker-publish.yml` (scan before sign).

**Issue:** CVE-2026-111943 / GO-2026-6809 (`crypto/x509` accepted certificate paths through untrusted intermediates) is fixed in Go 1.26.10. The static binary embeds the 1.26.9 standard library, so Trivy's image scan reports a HIGH finding with a fix available. `govulncheck` finds
no reachable call into the vulnerable code (this server imports neither `crypto/tls` nor `crypto/x509`), so the code-level gate was green and the image-level gate was red.

**Impact:** none for the server's behaviour (unreachable), but the scan-before-sign rule (F9) fails every release whose binary carries a fixable HIGH, and every web image built `FROM` static-server (eleven consumers) inherits the binary and its finding: a stale stdlib here would have blocked
signing of all of them.

**Remediation / evidence:** 0.1.11 bumps the toolchain to Go 1.26.10 (`go.mod`, the Dockerfile builder digest, the CI toolchain pin together; the Dockerfile fails if the builder's Go differs from the `go.mod` toolchain, F2). The gate was not weakened and no ignore entry was added. The same bump was made in
platform-probe 0.1.9, registry-token-service 0.1.9 and registry-auth-proxy 0.1.9 (their audits). Proof: run 39764402301 for `v0.1.11` succeeded in the verify jobs (govulncheck, image build and smoke test, Trivy filesystem scan, lint, tests) and in `Publish (quay.io + Docker Hub)`, which scans the pushed image before it signs (job conclusions read with `gh`, 2026-10-22; the best-effort self-hosted-registry mirror job failed, as recorded for earlier releases). Lesson recorded: a Go patch release with a stdlib advisory is a release trigger for this image
even when govulncheck says the code is unaffected, because the image scan reads the binary's embedded version.

---

## Section A — Invariant verification matrix

| # | Invariant (source) | Enforced at | Proven by | Status |
| --- | --- | --- | --- | --- |
| A1 | Cascade filesystem; never flattened (CLAUDE.md) | `cascadeFS` (`main.go:481`) | cascade tests | HOLDS |
| A2 | Directory listing disabled; `safeDir` outermost (CLAUDE.md) | `safeDir` (`:552`), `fileHandler` (`:258`) | listing tests | HOLDS |
| A3 | Served paths confined to the serve root (GO-M3; CLAUDE.md invariant 2) | `rootedFS` / `os.OpenRoot` (`:507`), `noDotfiles` (`:531`) | `TestSymlinkEscapeRefused`, `TestDotfilesHidden`, `TestRootedFSMissingDirServesNothing`, traversal and encoded-traversal tests | HOLDS (F1 RESOLVED) |
| A4 | Security headers on every route; never bypassed (CLAUDE.md) | `secureHeaders` outside `methodGuard` and the mux (`:585`, `newHandler` `:233`) | header tests, `TestSecurityHeadersOnMethodNotAllowed`, the smoke test | HOLDS (F12 RESOLVED; the image ships a restrictive default CSP since 0.1.9 — F4, RESOLVED) |
| A5 | GET/HEAD only | `methodGuard` (`:461`) | method tests | HOLDS |
| A6 | Immutable caching never applied to errors or navigations | `setCacheControl` + stdlib `serveError` + `no-cache` overrides + `serveIndex` | `TestNoCachingHeadersOnErrors`, `TestSPAWithoutIndexIsPlain404`, the smoke test (fails under `GODEBUG=httpservecontentkeepheaders=1`) | HOLDS (S1 done; stdlib-dependent, pinned) |
| A7 | Server timeouts; graceful shutdown (GO-M1, GO-M8) | `http.Server` (`:177`); signal handler | inspection | HOLDS (30-second write cap — F7) |
| A8 | CI gates on the released commit (GO-M7) | `docker-publish.yml` `verify` → `go-ci.yml` | Publish run 37964221333; the unpublished `0.1.5` | HOLDS (F2 RESOLVED) |
| A9 | Signatures verifiable against a pinned identity (IMG-M7) | release summary; SECURITY.md | inspection; `verify-digests.sh` 16/16 | HOLDS (F3 RESOLVED) |
| A10 | Docs discipline (CLAUDE.md) | every reference surface | `TestDocsListEveryOption` | HOLDS (F6 RESOLVED) |
| A11 | The binary is built with the toolchain CI checked (IMG scan-before-sign) | `go.mod` `toolchain` + Dockerfile `grep` | build fails on mismatch; Publish run | HOLDS (F2, F11 RESOLVED) |
| A12 | Pushed image scanned before it is signed | "Scan the published image" step before "Sign images" | Publish run steps in order | HOLDS (F9 ADJUDICATED: push precedes scan) |
| A13 | The image builds and runs (nonroot, probes, headers) before a tag | `image` job in `go-ci.yml`, reused by `verify` | `scripts/smoke.sh` (local Podman run 2026-10-18) | HOLDS (F10 RESOLVED) |
| A14 | A multi-arch release emulates nothing, and each platform's image holds that platform's binary | builder `FROM --platform=$BUILDPLATFORM`; `GOOS=$TARGETOS GOARCH=$TARGETARCH` with no default; a `go version -m` step asserts `GOARCH` equals the target; no `setup-qemu-action` in `docker-publish.yml` | rootless podman with no binfmt handler: both platforms built from the 0.1.9 sources and every file of both images equals the published 0.1.9 (`/server` amd64 `57a4338b…`, arm64 `f0a421b2…`, ELF aarch64) | HOLDS (0.1.10) |

---

## Section B — Supply-chain, publish-authority & capability matrix

### B.1 Dependency & CVE risk

| Component | Version | Status |
| --- | --- | --- |
| Go stdlib | go.mod `toolchain go1.26.9`; CI (setup-go) and image both 1.26.9; 1.26.9 is still the latest 1.26 patch (go.dev, 2026-10-18; Go 1.27.2 exists, not adopted) | govulncheck v1.8.0 green in the `v0.1.7` Publish run and locally on Go 1.26.9 (2026-10-18, no vulnerabilities) (F11) |
| Go modules | none (`go.mod` has no `require`; no `go.sum`) | nothing to scan |
| `golang:1.26.9-bookworm` (builder) | `sha256:d9c68c2c…` | builder only; nothing copied except the binary and rendered `public/` |
| Runtime | `scratch` | no OS packages |
| Image scanner | Trivy (`aquasecurity/trivy-action` v0.36.0): filesystem scan in CI; image scan in release | green in the `v0.1.7` Publish run (CRITICAL/HIGH, `ignore-unfixed`) |
| Dependabot | `.github/dependabot.yml`: docker and github-actions, weekly, grouped | no `gomod` entry needed (no modules); builder bumps are checked by the Dockerfile toolchain assertion (F10) |
| First-party runtime image | this is the first-party runtime image (the current release is `0.1.9@sha256:af6cc284…`, released 2026-10-19; the deployed consumer images are still built on `0.1.8@sha256:e32cf42b…`) | all eleven consumers' `Dockerfile`s on `main` pin `0.1.9` (read from GitHub 2026-10-19); none re-released |

### B.2 Publish authority & CI

| Authority | Where | Custody | Gates |
| --- | --- | --- | --- |
| Push to quay.io / Docker Hub | `docker-publish.yml` (tag `v*`) | repo secrets (`QUAY_*`, `DOCKERHUB_*`) — inventory is the maintainer item "Image registry credentials" in `OPERATOR_TASKS.md` | `verify` = full Go CI; image scan (F2) |
| Signing / attestation | same | OIDC keyless (`id-token: write` on the publish job only) | — |
| Self-hosted registry | same | secrets | `continue-on-error` (best-effort `mirror-private`: copies the signed public image by digest, no rebuild); its login fails without credentials (maintainer item, same `OPERATOR_TASKS.md` row) |

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

### C.1 Coverage grade — A (47 tests, `-race`, 93.0%; 2026-10-19)

| Dimension | Assessment |
| --- | --- |
| Happy path | Cascade, SPA, clean URLs, 404 page, immutable prefix, precompressed, headers, nonce, healthz, version |
| Error path | 405; missing files; listing refusal; traversal (plain and encoded); symlink escape; dot-files; unopenable root |
| Boundary | `q=0` and `Vary` (F5), a 404 under the immutable prefix (S1), `script-src-elem` and `'none'` (F4), the 405 headers (F12), the SPA 404 without `index.html` (F7): all covered in 0.1.8; the image's default CSP, its binary opt-out and the default page under it (F4): covered in 0.1.9 (`TestImageDefaultCSP`, `TestBinaryCSPDefaultIsUnset`, `TestDefaultPageFitsDefaultCSP`, `scripts/smoke.sh`). **Missing:** none known |
| Security-relevant | Strong for path syntax, path confinement and headers |

### C.2 Hermetic vs. live paths

| Path | Hermetic? | Deferred to | Tracking |
| --- | --- | --- | --- |
| The image as built (nonroot, healthcheck, served headers) | partly | `scripts/smoke.sh` in the `image` job of `go-ci.yml` (PR CI and the release gate), run against the built image; locally with rootless Podman (2026-10-18) | F10, S2 |
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

- [x] docs discipline restored — F6 RESOLVED (0.1.8, commit `ed7c737`; every option in every reference surface, the comment slips fixed, `TestDocsListEveryOption`)
- [x] regression test pinning "no caching headers on errors" (S1) and a container smoke test (S2, F10) — F10 RESOLVED (0.1.8, commit `ed7c737`; `TestNoCachingHeadersOnErrors`, `scripts/smoke.sh`, the `image` job in `go-ci.yml`; Podman run 2026-10-18)
- [x] CSP default and nonce handling decided — F4 RESOLVED / OQ2 decided (2026-10-19, option (c)): the nonce handling in 0.1.8 (`script-src-elem`, `'none'`) and the restrictive image-only default CSP in 0.1.9 (commit `e2a9aed`; `TestImageDefaultCSP`, `TestDefaultPageFitsDefaultCSP`, `scripts/smoke.sh`)
- [ ] external review — maintainer item (`OPERATOR_TASKS.md` "Funding, grants and an external audit")

---

## Cross-project themes

- **One runtime, every front end.** Header and cache semantics here are the VUE-M8 baseline for all
  apps. F1 was fixed once for every host (all eleven consumers moved to `0.1.7`, and to `0.1.8` on 2026-10-18; on `main` they pin `0.1.9` since 2026-10-19, not yet redeployed); F4 stays a
  configuration choice that every consumer makes (CSP in each Dockerfile), now over a restrictive image default (0.1.9) that each of them replaces.
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
- **Deployment readiness.** Section D is current; the one unticked item is the external review.
- **Pre-v0.2 policy:** F4, F5 and F6 may change defaults without shims (F4's new image default is such a change, with the override documented). 0.1.8 was released on 2026-10-18 and
  the consumers moved to the new base together (all eleven deployed on 0.1.8; 0.1.9 was released on 2026-10-19 and the eleven `Dockerfile`s pin it on `main` without a re-release). The 0.1.8 changes are
  additive or tightening for them: the 405 gains the headers, the nonce also goes in `script-src-elem` (none
  uses it), and `PRECOMPRESSED` negotiation is stricter (none enables it).

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
   consumer enables `PRECOMPRESSED` — **holds** (F5, RESOLVED in 0.1.8).
6. MUST keep security headers on every response, the 405 included, GET/HEAD only, and directory listing
   off — **holds** (A2, A4, A5; F12).
7. MUST NOT attach a caching header to an error response — **holds** (A6; `TestNoCachingHeadersOnErrors`).

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
| IMG-M3 | yes (stdlib, no downloads; `CGO_ENABLED=0`; `sed` in the builder, no `apt-get`; since 0.1.10 the builder runs on the build host and cross-compiles, so the release has no QEMU or privileged binfmt step) | A14 |
| IMG-M4 | yes (no secrets in args or layers) | front matter |
| IMG-M5 | yes (image `USER 65534`; consumer pods enforce `runAsNonRoot`, read-only root, no escalation, dropped capabilities, `RuntimeDefault`, no service-account token) | front matter; pod specs |
| IMG-M6 | yes for consumers (digest pins from `config/images.yaml`) | deployment status |
| IMG-M7 | yes (signature, SPDX attestation, provenance; the verification identity is pinned) | F3, B.IMG-1 |
| IMG-M8 | yes (scan before sign; SBOM from the image; pinned identity; no redistributed third-party code, so no licence notices beyond 0BSD) | F2, F9 |

**VUE hosting rows (the runtime's side of B.VUE-1 / VUE-M8):** `X-Content-Type-Options: nosniff`,
`X-Frame-Options: SAMEORIGIN`, `Referrer-Policy` always; HSTS and `Permissions-Policy` by default (an
empty variable omits them); CSP by default in the image (restrictive same-origin, since 0.1.9; none in the bare binary unless configured), with `script-src` nonce handling. The effective CSP
per consumer is that consumer's audit; the image's own default page is served under the image default (F4).

## Implementation suggestions (SHOULD / MAY)

- **S1** (done in 0.1.8: `TestNoCachingHeadersOnErrors`) SHOULD add a test asserting that a missing file under `CACHE_IMMUTABLE_PREFIX` returns 404
  **without** `Cache-Control`, so a Go upgrade or `GODEBUG=httpservecontentkeepheaders=1` cannot
  silently reintroduce year-long caching of 404s across every app.
- **S2** (done in 0.1.8: `scripts/smoke.sh`, the `image` job) SHOULD add a PR-time `docker build` and a container smoke test (run the image; check nonroot,
  `/healthz`, the headers and a 404) in `go-ci.yml` so `verify` also covers it (F10).
- **S3** (met differently in 0.1.8: `TestDocsListEveryOption` fails when an option is undocumented; no
  generator) MAY generate `public/llms.txt` and the default page's option table from one source (the
  package comment or a table file) to stop F6 recurring.
- **S4** MAY repeat the identity-pinned `cosign verify` command in the README and, when the cluster
  policy moves to enforce, pin it per repository (OQ3).

## Open questions (`OQ#`)

1. **OQ1** F1: are any consumer deployments mounting volumes into `SERVE_DIR` (ConfigMaps), and is
   `automountServiceAccountToken` disabled for these pods? (2026-10-09 evidence: `automountServiceAccountToken:
   false` is set on every consumer pod; no consumer deployment mounts a volume into
   `/app/public`: the only `mountPath` in the ten consumer manifests is the status deployment's
   `/etc/platform-probe`. The question stays open for the maintainer to confirm no out-of-tree deployment does.)
   **Decided 2026-10-22: closed. Nothing mounts into `SERVE_DIR`** (re-read: the in-tree consumer manifests mount nothing there, and the image's `/app/public` is baked at build time); an out-of-tree operator who mounts content owns that choice, and F1's confinement (`os.OpenRoot`) already bounds what a mount can expose.
2. **OQ2** F4: should the image ship a strict default CSP, which consumers override, rather
   than none? **DECIDED 2026-10-19 by the maintainer: option (c), implemented in 0.1.9** (reason: it protects every
   stock-image deployment, including the image's own page, with no binary change, no shims, the eleven consumers
   untouched because each sets its own CSP, and a break-glass in the operator's hands: set
   `CONTENT_SECURITY_POLICY` to their policy or to an empty string. Residual cost, accepted: out-of-tree
   operators whose mounted content uses inline scripts, inline styles or CDN assets have it blocked until they
   set their own policy; the bare binary still has no default.) The options as recorded 2026-10-18 before the
   decision: (2026-10-18, left open: more than one defensible default exists. Evidence: all eleven consumer
   Dockerfiles set `CONTENT_SECURITY_POLICY` themselves, so none would notice a new default; the unknown
   cost is out-of-tree operators mounting content with inline scripts, inline styles or CDN assets, which
   a default would block. Options: (a) status quo, no CSP unless set; (b) a binary default when the variable
   is absent, switched off by setting it to `""` (needs `envOrEmpty`); (c) an image-level `ENV` default in the
   Dockerfile only, e.g. `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:;
   object-src 'none'; base-uri 'none'; frame-ancestors 'self'`, which the binary leaves unset when run bare and
   an operator overrides or clears with an empty value; (d) a `<meta http-equiv>` CSP in the default page
   only, which would also block an edge-injected inline script on that page because the meta policy carries
   no nonce. **Recommendation: (c)**, with the default page checked against it in the smoke test; it needs no
   binary change, applies pre-v0.2 without shims, leaves the eleven consumers untouched, and puts the
   break-glass in an operator's hands. The maintainer decides whether unknown operators justify it.)
3. **OQ3** F3: does the cluster admission policy (if any) verify static-server-based images against
   a pinned identity? (2026-10-09 evidence: a `ClusterImagePolicy` exists in `warn` mode and pins the GitHub
   issuer plus any `meddleware-org` workflow; whether to enforce it, and per repository, is a platform decision.)
   **Decided 2026-10-22: enforce the `ClusterImagePolicy` with the workflow-anchored identity** (issuer `https://token.actions.githubusercontent.com`, subject anchored to the repository's `docker-publish.yml` on `refs/tags/v*`, as F3 prints), not any `meddleware-org` workflow. It is applied from the start on the server rebuild; **it is not enforced now** (the live policy is still in `warn` mode), so F3's claim stays "the identity is printed and verifiable", not "admission enforces it".

## Risks

- **Blast radius:** a defect here (headers, caching, confinement) reaches every Meddleware host at
  once. All eleven consumers are deployed on `0.1.8`-based images and pin `0.1.9` on `main` (not yet re-released), but every fix still needs a coordinated base bump.
- **Implicit stdlib guarantees:** the cache safety of 404s rests on Go's error-path behaviour; since 0.1.8
  a test pins it, so a toolchain or `GODEBUG` change fails the build instead of shipping.
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
    (all verify jobs, scan, sign, SBOM, provenance success; the best-effort private-registry job (then a rebuild, since replaced by the digest-preserving `mirror-private`) fails at login);
    Docker Hub digest and referrers; `config/images.yaml`; consumer Dockerfiles and pod specs.
  - **Not verified:** govulncheck and golangci-lint locally (only through the Publish run); quay.io manifest
    directly; the advisory texts for GO-2026-6607…6617.
  - **Contradiction with the workspace summary:** the Dockerfile has `USER 65534`, not `USER 65534:65534`;
    the group is 65534 through the `/etc/passwd` entry and consumer pods set `runAsUser: 65534`, so no gap.
  - **OQ1–OQ3** stay open with the evidence now in the questions.

- 2026-10-18 — fix wave on `main` = `v0.1.7` = `e1a71c5` plus local 0.1.8 (commit `ed7c737`; not tagged or
  published). Go 1.26.9 is still the latest 1.26 patch, so the toolchain and builder digest are unchanged.
  - **Resolved:** F5 (q-values and `Vary`: `acceptsEncoding`; three tests), F6 (five options in every
    surface, comment slips, `TestDocsListEveryOption`), F10 (`scripts/smoke.sh`, `image` job, `make smoke`),
    S1 (`TestNoCachingHeadersOnErrors`; fails under `GODEBUG=httpservecontentkeepheaders=1`), S2, S3 (as a test).
  - **New:** F12 RESOLVED (the 405 lacked the security headers; `secureHeaders` now wraps `methodGuard`;
    found by the smoke test's first run, which made the earlier "headers on every response" claim false for
    that status).
  - **Partly fixed:** F4 (nonce on `script-src-elem`, `'none'` left alone, `'unsafe-inline'` note; the
    default CSP stays OQ2 with options and a recommendation), F7 (`serveIndex` no longer keeps the immutable
    header on its 404; write timeout, SBOM version and build-arg escaping stay accepted: no single fix).
  - **Dockerfile user:** `USER 65534` kept (consumers set `runAsUser: 65534`); the smoke test asserts it.
  - **Gates:** pre-mainnet docs, S1/S2 and F10 ticked; the default-CSP decision (OQ2) and the external review stay open.
  - **Evidence of run:** `go vet` clean; gofmt clean; `go test -race -cover` ok (44 tests, 93.0%);
    golangci-lint v2.13.0 0 issues; govulncheck v1.8.0 none; the image built with rootless Podman and
    `scripts/smoke.sh` passed. Docker is unavailable here (root-only socket); the `image` job runs in
    GitHub Actions only.
  - **Not verified:** the CI `image` job on GitHub (first run on the next push).

- 2026-10-18 — released and deployed (`v0.1.8` = `3f1ddea`, `0.1.8@sha256:e32cf42b182d270c4ef57181e5588e7df454a42cac66e83d8ae59d9c7b7c4000`), verified live.
  - **Signature:** cosign keyless signature verified 2026-10-18 against the workflow-anchored identity
    (`docker-publish.yml@refs/tags/v*`, `verify-digests.sh`, 16/16 valid).
  - **Rollout:** consumers' images rebuilt on 0.1.8 and applied to the cluster 2026-10-18; every rollout
    succeeded. All eleven public pages answered 200 with the full header set (re-read with `curl -I`); the pages
    loaded in headless Chromium with no console errors or CSP violations (release facts).
  - **CI `image` job:** the "not verified" item of the fix-wave entry is closed: the `image` (build and smoke test) job passed in the `v0.1.8` Publish run 39115965481 (self-hosted-registry mirror job failed at login as before, best-effort).
  - **Open items unchanged:** OQ2 (default CSP) and the external review.

- 2026-10-19 — OQ2 decided by the maintainer (option (c)) and implemented as 0.1.9 (local commit at the time of this entry; released later the same day, see the release entry below).
  - **Resolved:** F4 (RESOLVED, was ACCEPTED-RISK): the Dockerfile sets the restrictive same-origin default
    `CONTENT_SECURITY_POLICY`; the Go default is unchanged; all eleven consumer Dockerfiles were grepped and each sets
    `ENV CONTENT_SECURITY_POLICY="${CSP}"` from a non-empty `ARG CSP` default, so none inherits the new default.
    Section D "CSP default and nonce handling decided" ticked; OQ2 marked decided with the residual cost for
    out-of-tree operators.
  - **Tests:** `TestImageDefaultCSP`, `TestBinaryCSPDefaultIsUnset`, `TestDefaultPageFitsDefaultCSP` (47 tests now);
    `scripts/smoke.sh` asserts the exact default plus a fresh nonce on a stock-image 200, 404 and `/healthz`, and that
    an empty value sends no CSP. `TestDocsListEveryOption` still passes.
  - **Docs:** README, AGENTS.md, llms.txt, `public/llms.txt`, `public/index.html`, `.env.example` (the variable is
    commented out there so an `--env-file` does not empty it), the package comment in `main.go`, CHANGELOG 0.1.9.
  - **Evidence of run:** gofmt and `go vet` clean; `go test -race -cover` ok (47 tests, 93.0%); golangci-lint v2.13.0
    0 issues; govulncheck v1.8.0 none (both built with and run under the `go.mod` toolchain 1.26.9); the image built
    with rootless Podman and `scripts/smoke.sh` passed; the default page loaded in headless Chrome against that
    image with `script.js` running and no CSP violation logged. Docker is unavailable here.
  - **Orchestrator next (done, see the release entry below):** tag and release 0.1.9; each of the eleven consumers keeps its own CSP, so a rebase
    onto 0.1.9 changes nothing for them (a consumer that ever drops its `ENV` would inherit the new default).
- 2026-10-19 — release status re-verified: **`v0.1.9` released** (tag = `30676d2`, code commit `e2a9aed`; Publish run 39181842677 green apart from the best-effort
  self-hosted-registry job; digest `sha256:af6cc284dace0d7bcacca61765f1e5efa0681da503c607f6e13ea35ce19edc72` read from the quay API (manifest list, 4 children present)
  and from Docker Hub; `config/images.yaml` pins the same digest).
  - **Consumers:** the `Dockerfile` of each of the eleven consumers on GitHub `main` pins `0.1.9@sha256:af6cc284…` (merged Dependabot docker PRs, which also bumped the `node:24-slim` digest);
    none was re-released or redeployed, because each sets its own CSP and nothing changes for them. The image itself is not deployed anywhere directly.
  - **Not re-run here:** the cosign verification (reported by the release orchestrator against `docker-publish.yml@refs/tags/v`; cosign is not installed in this sandbox).
  - **Findings and gates:** no change (F4 RESOLVED, OQ2 decided, pre-mainnet open only for the external review).
  - Dependabot policy encoded in .github/dependabot.yml (majors declined); open PRs merged 2026-10-19
  - `docker-publish.yml`: the private rebuild jobs are replaced by one `mirror-private` job that copies the published quay.io image (and its cosign artifacts) to the self-hosted registry by digest and fails unless the digest and the signature match (build once, promote by digest; committed locally, not yet released; the `PRIVATE_REGISTRY_*` credentials are still unset).
- 2026-10-18 — cosign identity in the workspace script: the default identity in `bootstrap/images/verify-digests.sh` is now workflow-anchored (`^https://github\.com/meddleware-org/${name}/\.github/workflows/docker-publish\.yml@refs/tags/v`); all 16 deployed and contingency images verified valid against it (`STRICT=0 bash bootstrap/images/verify-digests.sh`). This audit has no finding for it (F3 is RESOLVED on the published command); the F3 evidence sentence is updated. Unchanged: the cluster `ClusterImagePolicy` (OQ3, platform item).

- 2026-10-20 — image build moved off QEMU in 0.1.10 (commit e39de2e): the builder stage is `FROM --platform=$BUILDPLATFORM` and cross-compiles with `GOOS=$TARGETOS GOARCH=$TARGETARCH` (no default for either; the `RUN` aborts if one is empty; a `go version -m` step asserts the binary's `GOARCH` equals the target); `docker-publish.yml` has no `docker/setup-qemu-action` step, so no privileged binfmt step runs in the release. Proof with rootless podman (builder pinned by digest, no binfmt handler registered): the amd64 image built from the 0.1.9 sources equals the published 0.1.9 in every file (both architectures; the arm64 binary, `f0a421b2…`, is ELF aarch64 and equals the published one, so the QEMU build was already correct for this image: its `go build` line set no `GOARCH`). Checks: `go vet`, `go test -race` on go1.26.9, `scripts/smoke.sh` on the podman-built image. No finding changes. With this change all six Go and Node images of the fleet build natively (no emulated step remains).

- 2026-10-22 (audit close-out) — F13 added; OQ1 and OQ3 decided; release status.
  - **F13 (new, Low, RESOLVED):** Trivy flags the Go 1.26.9 stdlib (CVE-2026-111943, HIGH, fixed in 1.26.10) in the published binary although govulncheck finds it unreachable; the scan-before-sign gate would block every web image built on static-server. Resolved by the Go 1.26.10 bump in 0.1.11 (`v0.1.11` = `2ed0ebf`).
  - **Decided 2026-10-22:** OQ1 (nothing mounts into `SERVE_DIR`: closed), OQ3 (enforce the `ClusterImagePolicy` with the workflow-anchored identity; applied from the start on the server rebuild; not enforced now).
  - Counts: **10 RESOLVED, 1 ACCEPTED-RISK, 1 ADJUDICATED, 1 Positive** (F1–F13). Measured/verified: `v0.1.11` tag and the CHANGELOG entry read; the Go CI and the tag's publish run history was read, not re-run; no local test run in this pass.

## Pre-save consistency checklist (this pass)

- [x] Section A ↔ findings: A3 HOLDS (F1), A4 HOLDS (F12), A6 HOLDS (S1 done), A8 HOLDS (F2), A9 HOLDS (F3), A10 HOLDS (F6), A11 and A12 (F2, F11, F9), A13 (F10).
- [x] Finding header ↔ body: consistent (F6's struck-through line records the fixed SECURITY.md item).
- [x] Template line: base + GO + IMG + VUE (hosting rows) with the registry dates; untriggered lenses named.
- [x] Closing four-part structure present.
- [x] Open questions: OQ2 decided 2026-10-19; OQ1 and OQ3 decided 2026-10-22 (OQ3 not yet enforced).
- [x] Section D ↔ dispositions.
- [x] Executive summary ↔ dispositions (9 RESOLVED, 1 ACCEPTED-RISK, 1 ADJUDICATED, 1 Positive) and ceiling (High; realised Low at the first pass, Info now; F12 was Low and is RESOLVED).
- [x] C.1 counts measured 2026-10-19 (47 tests, 93.0%).
- [x] Re-verification log entry added.
- [x] 2026-10-22 (close-out): F13 added to the summary counts and the log; OQ1 and OQ3 carry their decisions.
