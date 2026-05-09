# Pipeline

`khealth` ships through four GitHub Actions workflows. Each is keyed to a
distinct trigger and a distinct concern, so a slow integration job never
blocks a fast lint signal on a doc-only PR.

| Workflow                | File                                     | Trigger                              | Purpose |
|-------------------------|------------------------------------------|--------------------------------------|---------|
| `ci`                    | [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)               | every PR + push to main              | lint, vet, race-tested unit tests, snapshot build, smoke-run the binary |
| `security`              | [`.github/workflows/security.yml`](../.github/workflows/security.yml)   | every PR, weekly cron                | govulncheck, gosec, CodeQL, Trivy image scan, gitleaks |
| `integration`           | [`.github/workflows/integration.yml`](../.github/workflows/integration.yml) | every PR + push to main              | spins up kind clusters across a K8s version matrix, runs `-tags=integration` tests against the built binary and image |
| `release`               | [`.github/workflows/release.yml`](../.github/workflows/release.yml)     | tag push `v*.*.*`                    | goreleaser → multi-arch binaries + Docker images + checksums + SBOM, signed with cosign |

## Local equivalent

Everything CI does is replayable locally via `make`:

```sh
make ci              # lint + vet + tests
make build           # multi-target binary into ./dist/
make security        # govulncheck + gosec
make integration     # kind cluster + integration tests
make release-snapshot # goreleaser snapshot (no publish)
```

## Pipeline graph

```
PR opened
   │
   ├─► ci (lint → test → build snapshot, smoke version + check cluster)
   ├─► security (govulncheck + gosec → SARIF; CodeQL; Trivy; gitleaks)
   └─► integration (matrix kind v1.30 / v1.31 → integration tests)

Tag pushed (v*.*.*)
   └─► release (goreleaser: archives + checksums + Docker images + SBOM + cosign)
```

## What CI publishes

- **Coverage profile** as a workflow artifact (`coverage` artifact in `ci`).
- **Snapshot binary** for download from any green PR (`khealth-snapshot`).
- **SARIF** uploaded to GitHub's "Security" tab for gosec, CodeQL, and Trivy.
- **Release artifacts** under GitHub Releases on tag push:
  - `khealth_<version>_<os>_<arch>.tar.gz` / `.zip`
  - `checksums.txt` (SHA-256)
  - SBOMs per archive (Syft, SPDX)
  - `ghcr.io/neilfarmer/k8s-health:<version>` multi-arch image
  - cosign signatures (keyless, OIDC-bound to the workflow)

## Hardening / future tightening

- **SLSA provenance** (`slsa-framework/slsa-github-generator`) for build
  provenance attestations on release.
- **Branch protection**: require `ci`, `integration`, and at least
  `govulncheck` + `codeql` to pass before merge.
- **Required signed commits** once the team is comfortable with the
  workflow.
- **Pin actions by SHA** (currently pinned by major/minor tag for readability;
  flip to `@sha256:...` once we accept the maintenance overhead).
