# ADR-0001: CLI framework — cobra + viper

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

`khealth` will have a multi-level command tree (`check pods`, `check nodes`,
`test run`, `test lint`, `config view`, ...) with shared global flags
(`--kubeconfig`, `--output`, `--namespace`). It needs:

- Subcommand routing.
- Flag parsing with environment-variable + config-file fallback (operators
  routinely set `KUBECONFIG`).
- Shell completion generation.
- Stable, well-documented behavior — `khealth` is going to be wrapped in
  pipelines, so surprising flag handling is a real cost.

The realistic options:

1. `spf13/cobra` (+ `spf13/viper` for config/env binding). De-facto standard in
   the Kubernetes ecosystem; `kubectl`, `helm`, `kubebuilder`, `argo` all use it.
2. `urfave/cli`. Lighter, but lacks first-class config-file binding and isn't
   the convention in this ecosystem.
3. Hand-rolled with `flag`. Ruled out — we'd reimplement subcommands and
   completion.

## Decision

Use **cobra for command/flag definition** and **viper for config-file and
environment-variable binding**. This matches the surrounding ecosystem and
buys us shell completion, structured help, and `--config` support without
writing it ourselves.

Precedence (highest wins): explicit flag → environment variable
(`KHEALTH_*`) → config file (`~/.config/khealth/config.yaml`) → built-in
default.

## Consequences

**Positive**
- Operators already know the conventions (`-A`, `-n`, `-o json`, `--kubeconfig`).
- Completion scripts come for free.
- Adding a subcommand is a small, well-understood diff.

**Negative**
- Two extra direct dependencies. Both are mature and widely vendored.
- viper is opinionated about file discovery; we'll wrap it in a thin
  `internal/config` package so the rest of the codebase doesn't see it.

**Forecloses**
- Embedding `khealth` as a Go library is still possible, but the CLI layer
  is not the API surface — `internal/checks` and `internal/testrunner` are.
