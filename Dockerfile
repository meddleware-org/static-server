# ── Build stage ───────────────────────────────────────────────────────────────
# Base pinned by digest for reproducible builds; the tag is kept for readability.
# To bump: docker buildx imagetools inspect golang:1.26-bookworm --format '{{.Manifest.Digest}}'
FROM golang:1.26-bookworm@sha256:6ef6e30f0ea5c384f6d111cf856e024e3086bbdcb1779da3f3b3fbba0aea53d2 AS builder

WORKDIR /build

COPY go.mod ./
COPY *.go ./

# Inject build-time metadata. VERSION is the only value that changes per release.
# All other values default to the upstream project coordinates; forks override
# via --build-arg without touching this file.
ARG VERSION=dev
ARG VENDOR="Meddleware"
ARG DESCRIPTION="Minimal scratch container image serving static files via stdlib Go HTTP."
ARG SOURCE_URL="https://github.com/meddleware-org/static-server"
ARG DOCUMENTATION_URL="https://github.com/meddleware-org/static-server#readme"
ARG IMAGE_URL="https://quay.io/meddleware-org/static-server"

# index.html template variables — all identifying values in the default page.
# The template (public/index.html) uses ${VAR} placeholders; envsubst renders
# the final /app/default/index.html at build time. The raw template is preserved
# at /app/default/index.html.tpl for operator use in initContainers.
ARG SITE_TITLE="static-server"
ARG SITE_KEYWORDS="static-server, container, docker, kubernetes, go, golang, scratch, file server, oci, configmap"
ARG SITE_AUTHOR_URL="https://meddleware.co.uk"
ARG OG_URL=""
ARG IMAGE_REF="quay.io/meddleware-org/static-server"
ARG SCHEMA_LICENSE_URL="https://opensource.org/licenses/0BSD"

# Fully static binary: no libc, no CGO — runs in scratch.
# Stripping debug info (-s -w) reduces binary size from ~10 MB to ~5 MB.
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o server .

# Render the index.html template with the build-time ARG values using sed, which
# ships in the base image — no `apt-get install`, so the build is fully
# reproducible against the pinned base digest (no network fetch of a floating
# gettext-base version). Only the named vars are substituted, avoiding accidental
# replacement of unrelated ${...} patterns.
# NOTE: build-arg values must not contain '|' (the sed delimiter) or '&' (the sed
# whole-match backreference). The defaults and typical fork overrides never do.
COPY public/ /tmp/public/
RUN cp /tmp/public/index.html /tmp/public/index.html.tpl \
    && sed \
         -e "s|\${SITE_TITLE}|${SITE_TITLE}|g" \
         -e "s|\${DESCRIPTION}|${DESCRIPTION}|g" \
         -e "s|\${SITE_KEYWORDS}|${SITE_KEYWORDS}|g" \
         -e "s|\${VENDOR}|${VENDOR}|g" \
         -e "s|\${SITE_AUTHOR_URL}|${SITE_AUTHOR_URL}|g" \
         -e "s|\${OG_URL}|${OG_URL}|g" \
         -e "s|\${SOURCE_URL}|${SOURCE_URL}|g" \
         -e "s|\${IMAGE_URL}|${IMAGE_URL}|g" \
         -e "s|\${IMAGE_REF}|${IMAGE_REF}|g" \
         -e "s|\${VERSION}|${VERSION}|g" \
         -e "s|\${SCHEMA_LICENSE_URL}|${SCHEMA_LICENSE_URL}|g" \
         /tmp/public/index.html.tpl \
         > /tmp/public/index.html

# Minimal /etc/passwd entry so the container can declare a non-root user.
RUN printf 'nobody:x:65534:65534:nobody:/:/sbin/nologin\n' > /tmp/passwd

# ── Runtime stage ─────────────────────────────────────────────────────────────
# scratch: zero OS footprint. The binary is fully self-contained.
FROM scratch

# Re-declare ARGs after FROM so they are visible in this stage.
ARG VERSION=dev
ARG VENDOR="Meddleware"
ARG DESCRIPTION="Minimal scratch container image serving static files via stdlib Go HTTP."
ARG SOURCE_URL="https://github.com/meddleware-org/static-server"
ARG DOCUMENTATION_URL="https://github.com/meddleware-org/static-server#readme"
ARG IMAGE_URL="https://quay.io/meddleware-org/static-server"

LABEL org.opencontainers.image.title="static-server" \
      org.opencontainers.image.description="${DESCRIPTION}" \
      org.opencontainers.image.url="${IMAGE_URL}" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.documentation="${DOCUMENTATION_URL}" \
      org.opencontainers.image.vendor="${VENDOR}" \
      org.opencontainers.image.licenses="0BSD" \
      org.opencontainers.image.version="${VERSION}"

COPY --from=builder /tmp/passwd /etc/passwd
COPY --from=builder /build/server /server

# Baked-in default files live at /app/default so they are never replaced by
# operator volume mounts. SERVE_DIR (/app/public) is the mount point for
# operator content. FALLBACK_DIR (/app/default) catches any file not found
# in SERVE_DIR — enabling partial overlays (e.g. mount only index.html and
# let the baked-in style.css / script.js / llms.txt serve from the fallback).
# index.html.tpl is the raw envsubst template for initContainer use.
COPY --from=builder /tmp/public/ /app/default/

USER 65534

EXPOSE 8080

ENV PORT=8080
ENV SERVE_DIR=/app/public
ENV FALLBACK_DIR=/app/default

# Healthcheck for non-Kubernetes runtimes (Docker, Compose).
# The binary handles -healthcheck by dialling localhost/healthz and exiting 0/1.
# Kubernetes uses its own readiness/liveness probes and ignores this instruction.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/server", "-healthcheck"]

ENTRYPOINT ["/server"]
