#!/usr/bin/env bash
# Smoke test of a built static-server image: bash scripts/smoke.sh <image>
#
# Runs the image the way a cluster does (the image's own user, read-only root filesystem, no
# capabilities, no privilege escalation) and checks what must hold without any mounted content:
#   - the image is configured to run as 65534 (nobody);
#   - the in-image `-healthcheck` mode succeeds (it is what the HEALTHCHECK instruction runs);
#   - /healthz answers 200 "ok" and /version answers the VERSION the image was built with ("smoke");
#   - the baked-in default page is served from /app/default (so the non-root user can read it);
#   - security headers are present on a 200, a 404 and a 405 response, including HSTS and
#     Permissions-Policy, and the CSP carries a fresh script nonce when one is configured;
#   - a container started with NO CONTENT_SECURITY_POLICY serves the image's restrictive default CSP (plus
#     the nonce), an operator-set value replaces it, and an empty value sends no CSP at all;
#   - a missing file under CACHE_IMMUTABLE_PREFIX is a 404 with NO Cache-Control header;
#   - dot-files are not served, and only GET/HEAD are accepted.
#
# Build the image with `--build-arg VERSION=smoke` first (`make smoke` does both). The container
# engine is $CONTAINER_ENGINE, else docker when its daemon answers, else podman (rootless works).
# Needs curl; nothing else.
set -euo pipefail

image="${1:?usage: smoke.sh <image>}"

engine="${CONTAINER_ENGINE:-}"
if [ -z "$engine" ]; then
  if command -v docker > /dev/null 2>&1 && docker info > /dev/null 2>&1; then
    engine=docker
  elif command -v podman > /dev/null 2>&1; then
    engine=podman
  else
    echo "no usable container engine (set CONTAINER_ENGINE)" >&2
    exit 1
  fi
fi

fail() {
  echo "smoke test FAILED: $*" >&2
  exit 1
}

user="$("$engine" image inspect --format '{{.Config.User}}' "$image")"
[ "$user" = "65534" ] || fail "image runs as '${user}', want 65534"

work="$(mktemp -d)"
csp="default-src 'self'; script-src 'self'"
cid="$("$engine" run -d --rm --read-only --cap-drop ALL --security-opt no-new-privileges \
  -e CONTENT_SECURITY_POLICY="$csp" -e CACHE_IMMUTABLE_PREFIX=/assets/ \
  -p 127.0.0.1::8080 "$image")"
extra_cids=""
trap '"$engine" logs "$cid" 2>&1 | tail -20; for c in $cid $extra_cids; do "$engine" rm -f "$c" > /dev/null 2>&1 || true; done; rm -rf "$work"' EXIT

hostport="$("$engine" port "$cid" 8080/tcp | head -n 1)"
base="http://${hostport}"

# fetch <method> <path>: sets $code and writes $work/headers and $work/body.
fetch() {
  code="$(curl -sS -X "$1" -D "$work/headers" -o "$work/body" -w '%{http_code}' "${base}$2")"
}
# header <name>: prints the value of a response header from the last fetch (case-insensitive).
header() {
  { grep -i "^$1:" "$work/headers" || true; } | head -n 1 | cut -d: -f2- | tr -d '\r' | sed 's/^ *//'
}
# require_header <name> <pattern>: the header must exist and match the extended regex.
require_header() {
  local v
  v="$(header "$1")"
  [[ "$v" =~ $2 ]] || fail "$3 $4: header $1 is '${v}', want match for $2"
}
security_headers() { # <label> <path>
  require_header X-Content-Type-Options '^nosniff$' "$1" "$2"
  require_header X-Frame-Options '^SAMEORIGIN$' "$1" "$2"
  require_header Referrer-Policy '^strict-origin-when-cross-origin$' "$1" "$2"
  require_header Strict-Transport-Security '^max-age=31536000; includeSubDomains$' "$1" "$2"
  require_header Permissions-Policy '^camera=\(\), geolocation=\(\), microphone=\(\), payment=\(\)$' "$1" "$2"
  require_header Content-Security-Policy "^default-src 'self'; script-src 'self' 'nonce-[A-Za-z0-9+/]{22}=='\$" "$1" "$2"
}

# Wait for the server (the in-image healthcheck mode doubles as the readiness probe).
ready=""
for _ in $(seq 1 30); do
  if "$engine" exec "$cid" /server -healthcheck > /dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[ -n "$ready" ] || fail "the in-image -healthcheck never succeeded"

fetch GET /healthz
[ "$code" = 200 ] && [ "$(cat "$work/body")" = "ok" ] || fail "/healthz answered ${code}, want 200 'ok'"
security_headers GET /healthz

fetch GET /version
[ "$code" = 200 ] && [ "$(cat "$work/body")" = "smoke" ] || fail "/version answered ${code} '$(cat "$work/body")', want 200 'smoke' (build with --build-arg VERSION=smoke)"

fetch GET /
[ "$code" = 200 ] || fail "/ answered ${code}, want 200 (the baked-in default page)"
grep -q '<h1>static-server</h1>' "$work/body" || fail "/ is not the rendered default page"
require_header Cache-Control '^no-cache$' GET /
security_headers GET /

fetch GET /llms.txt
[ "$code" = 200 ] || fail "/llms.txt answered ${code}, want 200"

fetch GET /assets/missing.abc123.js
[ "$code" = 404 ] || fail "/assets/missing.abc123.js answered ${code}, want 404"
[ -z "$(header Cache-Control)" ] || fail "404 under the immutable prefix carries Cache-Control: $(header Cache-Control)"
security_headers GET /assets/missing.abc123.js

fetch GET /.env
[ "$code" = 404 ] || fail "/.env answered ${code}, want 404"
fetch GET /.git/config
[ "$code" = 404 ] || fail "/.git/config answered ${code}, want 404"

fetch POST /
[ "$code" = 405 ] || fail "POST / answered ${code}, want 405"
require_header Allow '^GET, HEAD$' POST /
security_headers POST /

# The stock image, with no CONTENT_SECURITY_POLICY set, must send its restrictive same-origin default
# (the Dockerfile ENV), with the script nonce added to script-src; an empty value must switch it off.
default_csp="default-src 'self'; script-src 'self' 'nonce-[A-Za-z0-9+/]{22}=='; style-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'self'; form-action 'self'"
default_re="^${default_csp}\$"
stock_cid="$("$engine" run -d --rm --read-only --cap-drop ALL --security-opt no-new-privileges \
  -p 127.0.0.1::8080 "$image")"
extra_cids="$stock_cid"
stock_base="http://$("$engine" port "$stock_cid" 8080/tcp | head -n 1)"
for _ in $(seq 1 30); do
  "$engine" exec "$stock_cid" /server -healthcheck > /dev/null 2>&1 && break
  sleep 1
done
for path in / /healthz /missing.html; do
  curl -sS -D "$work/headers" -o /dev/null "${stock_base}${path}"
  v="$(header Content-Security-Policy)"
  [[ "$v" =~ $default_re ]] || fail "stock image GET ${path}: Content-Security-Policy is '${v}', want the restrictive default"
done
[[ "$v" != *unsafe-inline* ]] || fail "the default CSP allows unsafe-inline"

empty_cid="$("$engine" run -d --rm --read-only --cap-drop ALL --security-opt no-new-privileges \
  -e CONTENT_SECURITY_POLICY= -p 127.0.0.1::8080 "$image")"
extra_cids="$extra_cids $empty_cid"
empty_base="http://$("$engine" port "$empty_cid" 8080/tcp | head -n 1)"
for _ in $(seq 1 30); do
  "$engine" exec "$empty_cid" /server -healthcheck > /dev/null 2>&1 && break
  sleep 1
done
curl -sS -D "$work/headers" -o /dev/null "${empty_base}/"
[ -z "$(header Content-Security-Policy)" ] || fail "CONTENT_SECURITY_POLICY= (empty) still sent a CSP: $(header Content-Security-Policy)"

echo "smoke test passed (${engine}, ${image})"
