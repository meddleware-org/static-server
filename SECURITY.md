# Security Policy

## Scope

This policy covers security issues in:

- The Go binary (`main.go`) — vulnerabilities in request handling, header injection, path traversal, or similar
- The container image build (`Dockerfile`) — issues arising from the base image or build configuration
- The published images at `quay.io/meddleware-org/static-server` and `docker.io/meddleware/static-server`

## Invariants

- **Served paths are confined to the serve root.** Each directory is opened with `os.OpenRoot`, so a
  symlink that leaves it is not followed (a 404). Symlinks that stay inside it (a ConfigMap mount's
  `..data` links) work.
- **Dot-files are not served.** Any path segment starting with `.` is a 404 (`.env`, `.git/`, backups),
  except a leading `/.well-known/`.
- **Directory listing is off**, and security headers are set on every response.
- **Releases are checked and signed.** The release workflow runs the CI checks (lint, race tests,
  govulncheck, Trivy filesystem scan) on the tagged commit and a Trivy scan of the pushed image, then
  signs it keylessly. Verify against THIS repository's workflow, not any GitHub identity:

  ```bash
  cosign verify \
    --certificate-identity-regexp '^https://github.com/meddleware-org/static-server/\.github/workflows/docker-publish\.yml@refs/tags/v' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    quay.io/meddleware-org/static-server@<digest>
  ```

It does not cover:

- Security issues arising from content mounted at `SERVE_DIR` by the operator
- Vulnerabilities in the Go standard library itself (report those to the [Go security team](https://go.dev/security))
- Misconfigurations in the operator's Kubernetes / Docker deployment

## Supported versions

Only the latest published image tag receives security fixes.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Report vulnerabilities by emailing **<security@meddleware.co.uk>**. Include:

- A description of the vulnerability and its impact
- Steps to reproduce or a proof-of-concept (if available)
- The image tag or commit SHA you tested against

You will receive an acknowledgement within **3 business days** and a resolution plan within **14 days** for confirmed issues. Critical issues (CVSS ≥ 9.0) are prioritised for same-day acknowledgement.

## Disclosure

Once a fix is released, a security advisory will be published on the GitHub repository. Reporters may be credited by name unless they prefer to remain anonymous.
