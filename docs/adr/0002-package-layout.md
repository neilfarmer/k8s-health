# ADR-0002: Single binary, `internal/`-first package layout

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

`khealth` is intended to be distributed as a **single static binary**.
Operators copy it into a container image, drop it on a bastion, or run it from
CI. We do not (yet) want to publish a Go library API — a public `pkg/` would
be a long-term commitment we can't yet honor.

We also need a clean home for code that *might* later become public:
result types, the check interface, the HealthTest schema.

## Decision

- `cmd/khealth/main.go` is the only program entry point.
- Everything else lives under `internal/`. Today's `internal/result` and
  `internal/checks` packages are private. If consumers later need them as
  libraries, we promote individual packages from `internal/` to `pkg/` with a
  versioning commitment at that time.
- Keep `pkg/` empty until we have a real external consumer — a discipline
  that prevents accidental API surface.

Layout is captured in [`architecture.md`](../architecture.md#proposed-package-layout).

## Consequences

**Positive**
- No accidental "I depended on this internal type" drama.
- Single binary, single install path; no plugin discovery (see
  [ADR-0004](0004-check-registry.md)).

**Negative**
- Anyone who wants to embed `khealth` checks today has to fork. Acceptable —
  there is no demand yet.

**Forecloses**
- A plugin model where third parties register checks via Go plugin or RPC.
  We may revisit this in Phase 3 if a real use case appears.
