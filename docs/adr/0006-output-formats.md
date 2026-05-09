# ADR-0006: Output formats and exit-code contract

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

`khealth` will be consumed by humans, shell pipelines, CI systems, monitoring
stacks, and chat tooling. They want different things:

- A human at a terminal: a colored table with red lines at the top.
- A `jq` pipeline: stable JSON.
- CI: `JUnit XML` so the CI surface "just works."
- Prometheus `textfile_collector`: `.prom` files written by a CronJob.
- Slack / webhook: a JSON POST.

We also need a consistent exit-code contract. Right now there isn't one;
`kubectl` only returns 0/1 and gives users very little to switch on.

## Decision

### Output formats

A single rendering step, fed by the unified `Report` type
([`architecture.md`](../architecture.md#result-model)):

| `--output` | Notes |
|------------|-------|
| `table` *(default)* | Color when stdout is a TTY, plain otherwise. Sections grouped by check category. |
| `json` | Stable schema; documented and versioned. Breaking changes bump a `schemaVersion` field. |
| `yaml` | Same shape as JSON. |
| `junit` | One `<testcase>` per finding; `WARN` → `<failure>` only with `--strict`. |
| `prom` | One gauge per check ID with `status` label; suitable for `textfile_collector`. |

Renderers live in `internal/render/` and implement a single `Render(io.Writer, Report) error`
method. The CLI does not let renderers reach into checks — they receive the
finished `Report` only.

### Exit codes

| Worst severity in report | Default | With `--strict` |
|--------------------------|---------|-----------------|
| All `OK` / `SKIP`        | 0       | 0               |
| `WARN` highest           | 0       | 2               |
| `CRIT` present           | 1       | 1               |
| Tool error               | 3       | 3               |

A "tool error" is anything that prevented the report from being trustworthy:
auth failure, kubeconfig parse error, fatal panic in the runner. We
deliberately separate tool errors (`3`) from cluster errors (`1`) so CI can
distinguish "your build cluster is broken" from "your code's deployment is
sick."

## Consequences

**Positive**
- Predictable, scriptable. `if khealth check cluster --strict; then ...`
  has well-defined semantics.
- New output formats are additive: implement `render.Renderer`, register it.
- The `prom` format gives operators a path to long-term trending without
  building a daemon.

**Negative**
- The JSON schema becomes part of the public contract. We commit to
  `schemaVersion` and document it.

**Forecloses**
- We do *not* support templated/custom output (`--output go-template=`). It's
  attractive but doubles the surface area we have to keep stable. Users who
  want bespoke shapes can `jq` over `--output json`.
