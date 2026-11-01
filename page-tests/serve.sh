#!/usr/bin/env bash
# Serve the default page the way the image does: render the ${VAR} placeholders of public/index.html with sample values (the Dockerfile
# does this with sed), put the result in a fallback directory with the other default files, and run the server on it.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp "$root"/public/* "$work"/
sed -E \
  -e 's#\$\{SITE_TITLE\}#static-server#g' \
  -e 's#\$\{DESCRIPTION\}#A minimal scratch container image that serves static files.#g' \
  -e 's#\$\{SITE_KEYWORDS\}#static, server, container#g' \
  -e 's#\$\{VENDOR\}#Meddleware#g' \
  -e 's#\$\{(SOURCE_URL|OG_URL|SITE_AUTHOR_URL|SCHEMA_LICENSE_URL)\}#https://example.org/#g' \
  -e 's#\$\{IMAGE_URL\}#https://example.org/image#g' \
  -e 's#\$\{IMAGE_REF\}#example.org/static-server#g' \
  -e 's#\$\{VERSION\}#v0.0.0#g' \
  "$root/public/index.html" > "$work/index.html"
cd "$root"
mkdir -p "$work/empty"
PORT="${PORT:-8089}" SERVE_DIR="$work/empty" FALLBACK_DIR="$work" exec go run .
