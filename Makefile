IMAGE   := static-server
VERSION ?= dev

# ── Registry coordinates ──────────────────────────────────────────────────────
# Each registry and its namespace are independently overridable.
# Set REGISTRIES wholesale to restrict publishing to a subset.
QUAY_REGISTRY       ?= quay.io
QUAY_NAMESPACE      ?= meddleware-org
DOCKERHUB_REGISTRY  ?= docker.io
DOCKERHUB_NAMESPACE ?= meddleware
PRIVATE_REGISTRY    ?= registry.meddleware.co.uk
PRIVATE_NAMESPACE   ?= meddleware-org

REGISTRIES ?= \
  $(QUAY_REGISTRY)/$(QUAY_NAMESPACE) \
  $(DOCKERHUB_REGISTRY)/$(DOCKERHUB_NAMESPACE) \
  $(PRIVATE_REGISTRY)/$(PRIVATE_NAMESPACE)

FULL_TAGS := $(foreach r,$(REGISTRIES),$(r)/$(IMAGE):$(VERSION))

# ── OCI image spec metadata ───────────────────────────────────────────────────
# Passed as --build-arg so forks override via make variables, not Dockerfile edits.
VENDOR            ?= Meddleware
DESCRIPTION       ?= Minimal scratch container image serving static files via stdlib Go HTTP.
SOURCE_URL        ?= https://github.com/meddleware-org/static-server
DOCUMENTATION_URL ?= https://github.com/meddleware-org/static-server\#readme
IMAGE_URL         ?= https://quay.io/meddleware-org/static-server

# ── index.html template variables ────────────────────────────────────────────
# envsubst renders these into /app/default/index.html at build time.
# The raw template is preserved at /app/default/index.html.tpl.
SITE_TITLE        ?= static-server
SITE_KEYWORDS     ?= static-server, container, docker, kubernetes, go, golang, scratch, file server, oci, configmap
SITE_AUTHOR_URL   ?= https://meddleware.co.uk
OG_URL            ?=
IMAGE_REF         ?= quay.io/meddleware-org/static-server
SCHEMA_LICENSE_URL ?= https://opensource.org/licenses/0BSD

BUILD_ARGS := \
  --build-arg VERSION=$(VERSION) \
  --build-arg VENDOR="$(VENDOR)" \
  --build-arg DESCRIPTION="$(DESCRIPTION)" \
  --build-arg SOURCE_URL=$(SOURCE_URL) \
  --build-arg DOCUMENTATION_URL=$(DOCUMENTATION_URL) \
  --build-arg IMAGE_URL=$(IMAGE_URL) \
  --build-arg SITE_TITLE="$(SITE_TITLE)" \
  --build-arg SITE_KEYWORDS="$(SITE_KEYWORDS)" \
  --build-arg SITE_AUTHOR_URL=$(SITE_AUTHOR_URL) \
  --build-arg OG_URL=$(OG_URL) \
  --build-arg IMAGE_REF=$(IMAGE_REF) \
  --build-arg SCHEMA_LICENSE_URL=$(SCHEMA_LICENSE_URL)

.PHONY: build push build-multi test vet run

## build: build image locally for the current platform
build:
	docker build $(BUILD_ARGS) $(foreach t,$(FULL_TAGS),-t $(t)) .

## push: push already-built local tags to all registries
push:
	$(foreach t,$(FULL_TAGS),docker push $(t);)

## build-multi: build and push multi-platform image (amd64 + arm64) to all registries
build-multi:
	docker buildx build --platform linux/amd64,linux/arm64 \
	  $(BUILD_ARGS) $(foreach t,$(FULL_TAGS),--tag $(t)) --push .

## test: run unit tests with race detector
test:
	go test -v -race ./...

## vet: run go vet
vet:
	go vet ./...

## run: run the server locally (serves public/ on :8080)
run:
	go run .
