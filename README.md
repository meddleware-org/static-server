# static-server

A minimal scratch container image (~5 MB) that serves static files via a fully static Go binary.

- **Zero OS footprint** — built on `scratch`; no shell, no package manager, no libc
- **Non-root** — runs as UID 65534 (`nobody`)
- **No external dependencies** — pure Go standard library; `CGO_ENABLED=0`
- **Kubernetes-native** — `GET /healthz` probe, graceful SIGTERM shutdown, ConfigMap-mountable content
- **Security headers** — `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` on every response; HSTS and `Permissions-Policy` by default; a restrictive same-origin CSP by default in the image (any CSP when configured)
- **0BSD licence** — do whatever you want

## Quick start

```bash
# Run with the default landing page
docker run -p 8080:8080 quay.io/meddleware-org/static-server:latest

# Serve your own content
docker run -p 8080:8080 \
  -v "$(pwd)/dist:/app/public:ro" \
  quay.io/meddleware-org/static-server:latest
```

## Container registries

```text
quay.io/meddleware-org/static-server:latest
quay.io/meddleware-org/static-server:v0.1.0
docker.io/meddleware/static-server:latest
docker.io/meddleware/static-server:v0.1.0
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | TCP port the server listens on |
| `SERVE_DIR` | `/app/public` | Primary directory to serve static files from; mount operator content here |
| `FALLBACK_DIR` | `/app/default` | Fallback directory when a file is not found in `SERVE_DIR`; set to empty to disable |
| `SPA_FALLBACK` | `false` | Serve `index.html` (200) for unknown navigation routes (extensionless paths); asset paths still 404 on miss |
| `CLEAN_URLS` | `false` | Serve `/<path>` from `/<path>.html` when `/<path>` itself does not exist (static-site generators with extensionless links, e.g. VitePress `cleanUrls`) |
| `NOT_FOUND_PAGE` | (empty) | Page served with status `404` for unknown navigation routes, e.g. `/404.html`; empty = plain 404. Ignored when `SPA_FALLBACK` is on; assets still get a plain 404 |
| `CACHE_IMMUTABLE_PREFIX` | (empty) | Path prefix whose files get one-year immutable `Cache-Control` (fingerprinted assets, e.g. `/assets/`); html/nav get `no-cache` |
| `CONTENT_SECURITY_POLICY` | (unset in the bare binary); the **image** sets `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'self'; form-action 'self'` | Sets the `Content-Security-Policy` header when non-empty. The image's default is restrictive and same-origin (no inline script or style, no third-party origin, no plugins, no `<base>`); content that needs more (inline scripts or styles, CDN assets) must set its own policy. Override it by setting this variable to your policy; set it to an empty string to send no CSP. Consumer images set their own and are unaffected |
| `CSP_NONCE` | `true` | Appends a fresh `'nonce-…'` to the policy's `script-src` and `script-src-elem` on every response. The server inlines no scripts; the nonce lets an edge that injects scripts and reads the nonce from this header (Cloudflare JavaScript Detections / Bot Fight Mode) run them without `'unsafe-inline'`. A policy with neither directive is left unchanged, as is a directive that is `'none'`. **A nonce makes browsers ignore `'unsafe-inline'` in the directive it joins**: if your own pages rely on inline scripts, allow them by hash or set `CSP_NONCE=false`. |
| `STRICT_TRANSPORT_SECURITY` | `max-age=31536000; includeSubDomains` | `Strict-Transport-Security` value; set to empty to omit the header |
| `PERMISSIONS_POLICY` | `camera=(), geolocation=(), microphone=(), payment=()` | `Permissions-Policy` value; set to empty to omit the header |
| `PRECOMPRESSED` | `false` | Serve a sibling `.br`/`.gz` asset when the client accepts it (a coding refused with `q=0` is not served) and it exists. Asset responses carry `Vary: Accept-Encoding` |

See [`.env.example`](.env.example) for a copy-paste template.

### Hosting a generated static site

For a multi-page static site (VitePress, Astro, Hugo …) set `CLEAN_URLS=true` and
`NOT_FOUND_PAGE=/404.html`, and leave `SPA_FALLBACK` off: every page is served from its own HTML
file, and an unknown path gets the site's 404 page with a real `404` status (an SPA fallback would
answer every unknown path with the home page and `200`).

### Hosting a single-page app (SPA)

Set `SPA_FALLBACK=true` so client-side routes and deep-link refresh resolve to `index.html`,
and `CACHE_IMMUTABLE_PREFIX=/assets/` so fingerprinted bundles cache immutably while
`index.html` stays `no-cache`. Correct MIME types (incl. `.wasm` → `application/wasm`) are
registered automatically. Only `GET`/`HEAD` are accepted (others get `405`).

## Endpoints

| Method | Path | Status | Description |
| ------ | ---- | ------ | ----------- |
| `GET` | `/` | 200 / 404 | Serves `SERVE_DIR/index.html`; 404 if absent |
| `GET` | `/<path>` | 200 / 404 | Serves file; 404 if missing or a directory without `index.html` |
| `GET` | `/healthz` | 200 | Liveness / readiness probe; body: `ok` |
| `GET` | `/version` | 200 | Build-time version string as plain text |
| `GET` | `/llms.txt` | 200 | Machine-readable API reference for AI/LLM agents |

Directory listing is disabled; a directory without `index.html` returns 404.

## Kubernetes deployment

### ConfigMap mount (recommended for single-page apps)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: static-server
          image: quay.io/meddleware-org/static-server:latest
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet: { path: /healthz, port: 8080 }
          livenessProbe:
            httpGet: { path: /healthz, port: 8080 }
          volumeMounts:
            - name: public
              mountPath: /app/public
              readOnly: true
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities: { drop: [ALL] }
          resources:
            requests: { cpu: 5m, memory: 16Mi }
            limits:   { cpu: 100m, memory: 32Mi }
      volumes:
        - name: public
          configMap:
            name: my-app-html
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-app-html
data:
  index.html: |
    <!doctype html><html><body>hello</body></html>
```

### kustomize ConfigMap generator

```yaml
# kustomization.yaml — colocate with your index.html
configMapGenerator:
  - name: my-app-html
    files:
      - index.html
generatorOptions:
  disableNameSuffixHash: true
```

### Partial overlay (FALLBACK_DIR cascade)

The image ships baked-in defaults at `/app/default` (`FALLBACK_DIR`). Files not found in
`SERVE_DIR` automatically fall through to `FALLBACK_DIR`. This means you can mount only the
files you want to customise — the rest are served from the baked-in copies.

```yaml
# Mount only index.html via ConfigMap; style.css, script.js,
# llms.txt, and robots.txt are served from /app/default automatically.
spec:
  containers:
    - name: static-server
      image: quay.io/meddleware-org/static-server:latest
      volumeMounts:
        - name: public
          mountPath: /app/public/index.html
          subPath: index.html
          readOnly: true
  volumes:
    - name: public
      configMap:
        name: my-app-html
```

To disable the fallback entirely (serve only what is in `SERVE_DIR`):

```yaml
env:
  - name: FALLBACK_DIR
    value: ""
```

### initContainer (for build-step content or dynamic metadata)

Populate an emptyDir at startup from a build step or template rendering, then serve the result.
The static server remains a pure file server; the initContainer handles one-shot configuration.

```yaml
spec:
  initContainers:
    - name: build
      image: node:22-alpine
      command: [sh, -c, "npm ci && npm run build && cp -r dist/* /public/"]
      volumeMounts:
        - name: public
          mountPath: /public
  containers:
    - name: static-server
      image: quay.io/meddleware-org/static-server:latest
      volumeMounts:
        - name: public
          mountPath: /app/public
  volumes:
    - name: public
      emptyDir: {}
```

To inject deployment-specific metadata (canonical URL, `og:url`) without making the server
dynamic, use `envsubst` in the initContainer to render a template before the server starts:

```yaml
spec:
  initContainers:
    - name: configure
      image: alpine:3
      command:
        - sh
        - -c
        - |
          apk add --no-cache gettext
          envsubst < /template/index.html > /public/index.html
      env:
        - name: OG_URL
          value: "https://my-app.example.com/"
      volumeMounts:
        - name: template
          mountPath: /template
          readOnly: true
        - name: public
          mountPath: /public
  containers:
    - name: static-server
      image: quay.io/meddleware-org/static-server:latest
      volumeMounts:
        - name: public
          mountPath: /app/public
          readOnly: true
  volumes:
    - name: template
      configMap:
        name: my-app-template   # template with ${VAR} placeholders
    - name: public
      emptyDir: {}              # initContainer writes here; static-server reads it
```

## Building

### Prerequisites

- Docker with [buildx](https://docs.docker.com/buildx/working-with-buildx/) for multi-platform builds
- Go 1.26+ for local development and testing (`go.mod` names the exact toolchain)
- `curl` and Docker or Podman for the container smoke test

### Using the Makefile

```bash
make test                        # go test -race ./...
make vet                         # go vet ./...
make run                         # run locally on :8080
make smoke                       # build the image and smoke-test it (CONTAINER_ENGINE=podman works)

make build VERSION=v0.1.0        # build local single-platform image
make build-multi VERSION=v0.1.0  # build + push multi-platform to all registries
```

All registry coordinates and OCI metadata are Make variables with sensible defaults.
Override any without editing the Makefile:

```bash
# Publish to your own registry
make build-multi VERSION=v0.1.0 \
  QUAY_NAMESPACE=myorg \
  DOCKERHUB_NAMESPACE=myorg \
  VENDOR="My Org" \
  SOURCE_URL=https://github.com/myorg/static-server
```

### Direct docker commands

```bash
docker build --build-arg VERSION=v0.1.0 \
  -t quay.io/meddleware-org/static-server:v0.1.0 .

docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v0.1.0 --push \
  -t quay.io/meddleware-org/static-server:v0.1.0 .
```

### Build arguments

All identifying content in the image is configurable via `--build-arg` (or Makefile variables).
No file editing required — forks and operators pass only what differs from the upstream defaults.

**OCI image spec labels** (`org.opencontainers.image.*`):

| ARG | Default |
| --- | ------- |
| `VERSION` | `dev` |
| `VENDOR` | `Meddleware` |
| `DESCRIPTION` | Minimal scratch container image serving static files via stdlib Go HTTP. |
| `SOURCE_URL` | `https://github.com/meddleware-org/static-server` |
| `DOCUMENTATION_URL` | `https://github.com/meddleware-org/static-server#readme` |
| `IMAGE_URL` | `https://quay.io/meddleware-org/static-server` |

**index.html template variables** (substituted into the default page at build time with `sed`; the raw template keeps the `${VAR}` placeholders for `envsubst` in an initContainer):

| ARG | Default | Used in |
| --- | ------- | ------- |
| `SITE_TITLE` | `static-server` | `<title>`, `<h1>`, og:title, twitter:title, JSON-LD name |
| `DESCRIPTION` | (see above) | meta description, og:description, twitter:description, JSON-LD description |
| `SITE_KEYWORDS` | (see Dockerfile) | meta keywords |
| `VENDOR` | `Meddleware` | meta author, JSON-LD author name |
| `SITE_AUTHOR_URL` | `https://meddleware.co.uk` | JSON-LD author URL |
| `OG_URL` | `` (empty) | og:url — set to your deployment URL via initContainer |
| `SOURCE_URL` | `https://github.com/meddleware-org/static-server` | body links, JSON-LD codeRepository |
| `IMAGE_URL` | `https://quay.io/meddleware-org/static-server` | JSON-LD downloadUrl (with https scheme) |
| `IMAGE_REF` | `quay.io/meddleware-org/static-server` | docker run and Kubernetes YAML examples |
| `VERSION` | `dev` | JSON-LD version |
| `SCHEMA_LICENSE_URL` | `https://opensource.org/licenses/0BSD` | JSON-LD license |

The raw template is preserved at `/app/default/index.html.tpl` inside the image for initContainer use.

## CI / CD

| Workflow | Trigger | Action |
| -------- | ------- | ------ |
| `go-ci.yml` | push, pull_request, and called by the release | golangci-lint, `go vet` + `go test -race`, govulncheck, Trivy filesystem scan, and an image job that builds the image and runs `scripts/smoke.sh` against it |
| `docker-publish.yml` | tag push `v*` | runs `go-ci.yml`, then multi-platform build + push to quay.io and Docker Hub (scanned, cosign-signed, SBOM and provenance attested) and a best-effort self-hosted registry |

See [`.github/workflows/`](.github/workflows/) for the full workflow YAML. Registry credentials
and org coordinates are GitHub repository variables/secrets — no YAML edits needed for forks.

## Security

- The binary is fully static (`CGO_ENABLED=0`); no shared libraries can be compromised.
- The container runs as UID 65534 (nobody) with no capabilities.
- The root filesystem is intended to be mounted read-only (`readOnlyRootFilesystem: true`).
- No writable paths inside the container are required at runtime.

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Licence

[BSD Zero Clause (0BSD)](LICENSE) — do whatever you want, no attribution required.
