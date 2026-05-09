# ADR-0004: Check registry — small interface, no plugins (yet)

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

We need a way to add checks without touching unrelated code. The candidates:

1. A **single `Check` interface** with a compile-time registry inside the
   binary. Each check is a small file in `internal/checks/`.
2. A **runtime plugin model** (Go plugins, gRPC à la Terraform, or shelling
   out to external binaries).

Plugins are tempting because "anyone can add a check," but they bring real
costs: plugin compatibility, ABI stability, RBAC for arbitrary code, and
distribution complexity. None of the immediate users have asked for them.

## Decision

Adopt option 1: a small Go interface with an internal registry. Checks live
in `internal/checks/<id>.go`, register themselves in `init()`, and are
filtered at runtime by `--checks` / `--skip-checks`.

```go
// internal/checks/registry.go (illustrative)

type Check interface {
    ID() string                                // e.g. "pods.backoff"
    Description() string
    Requires() Capabilities
    Run(ctx context.Context, env Env) []result.Finding
}

var registry = map[string]Check{}

func Register(c Check) {
    if _, dup := registry[c.ID()]; dup {
        panic("duplicate check id: " + c.ID())
    }
    registry[c.ID()] = c
}

func All() []Check { /* ... */ }
func Filter(include, exclude []string) []Check { /* ... */ }
```

```go
// internal/checks/pods_backoff.go (illustrative)

func init() { Register(&podsBackoff{}) }

type podsBackoff struct{}

func (podsBackoff) ID() string          { return "pods.backoff" }
func (podsBackoff) Description() string { return "Pods in CrashLoopBackOff or ImagePullBackOff" }
func (podsBackoff) Requires() Capabilities { return CapAPIServer }

func (c podsBackoff) Run(ctx context.Context, env Env) []result.Finding {
    // list pods, walk container statuses, emit findings
}
```

The runner executes checks in a worker pool with per-check context timeouts.

## Consequences

**Positive**
- Adding a check is one file + one line of registration. PR diffs stay small.
- Test isolation is trivial — each check is a unit-testable struct.
- No plugin loader, no ABI to maintain.

**Negative**
- Third-party checks require building a fork. We accept this for now.
- One crashing check can take down the worker if we're not careful → mitigated
  with `recover()` in the runner and per-check timeouts.

**Forecloses (for now)**
- gRPC / external plugin model. We can revisit if there is real demand
  (Phase 3+).
