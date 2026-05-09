# Architecture

This document describes the proposed component layout for `khealth`. It is
illustrative — none of the code below has been written yet.

## High-level components

```
                      ┌──────────────────────────────────────┐
                      │              khealth CLI              │
                      │  (cobra commands, flag parsing, IO)   │
                      └──────────────────────────────────────┘
                                       │
        ┌──────────────────────────────┼──────────────────────────────┐
        ▼                              ▼                              ▼
┌───────────────┐             ┌────────────────┐             ┌───────────────┐
│ Check Registry│             │ Test Runner    │             │ Renderers     │
│  (pluggable)  │             │ (HealthTest)   │             │ table/json/.. │
└───────────────┘             └────────────────┘             └───────────────┘
        │                              │                              ▲
        ▼                              ▼                              │
┌───────────────────────────────────────────────────┐                 │
│            Kubernetes Client Layer                │                 │
│  client-go (typed + dynamic), discovery, RESTMapper│ ───── results ─┘
└───────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────┐
│  Control-plane Probes (etcd, apiserver, scheduler) │
│  (in-cluster Job mode for privileged reads)        │
└───────────────────────────────────────────────────┘
```

## Proposed package layout

```
github.com/neilfarmer/k8s-health
├── cmd/
│   └── khealth/
│       └── main.go                  // cobra root, version, signal handling
├── internal/
│   ├── cli/                         // cobra command definitions
│   │   ├── root.go
│   │   ├── check/                   // `khealth check ...`
│   │   ├── test/                    // `khealth test ...`
│   │   └── version/
│   ├── kube/                        // kubeconfig + in-cluster auth, clients
│   │   ├── client.go                // builds typed + dynamic clients
│   │   ├── discovery.go
│   │   └── mode.go                  // in-cluster vs out-of-cluster detection
│   ├── checks/                      // each check is a small, self-contained file
│   │   ├── registry.go              // Check interface + registry
│   │   ├── pods_backoff.go
│   │   ├── pods_pending.go
│   │   ├── nodes_ready.go
│   │   ├── deployments_rollout.go
│   │   ├── pvc_pending.go
│   │   ├── events_warnings.go
│   │   ├── apiserver_healthz.go
│   │   ├── scheduler_healthz.go
│   │   ├── controller_healthz.go
│   │   ├── coredns_resolution.go
│   │   ├── etcd_health.go           // launches in-cluster Job if needed
│   │   └── etcd_size.go
│   ├── testrunner/                  // HealthTest CRD-style runner
│   │   ├── spec.go                  // YAML schema (apiVersion: khealth.io/v1alpha1)
│   │   ├── decode.go
│   │   ├── apply.go                 // dynamic apply / delete with cleanup
│   │   ├── assert.go                // assertion engines (http, exec, log, dns)
│   │   └── runner.go                // orchestrates setup → steps → cleanup
│   ├── render/                      // output formats
│   │   ├── table.go
│   │   ├── json.go
│   │   ├── yaml.go
│   │   ├── junit.go
│   │   └── prom.go
│   └── result/                      // shared result types: Status, Severity, Finding
│       └── result.go
├── pkg/                             // (kept empty until something is genuinely reusable)
├── examples/
│   └── healthtests/
└── docs/
```

## Result model

Every check and every test step produces the same shape so the renderers don't
care which one emitted it:

```go
// internal/result/result.go (illustrative)

type Status string

const (
    StatusOK       Status = "OK"
    StatusWarning  Status = "WARN"
    StatusCritical Status = "CRIT"
    StatusUnknown  Status = "UNKNOWN"
    StatusSkipped  Status = "SKIP"
)

type Finding struct {
    Check     string            // e.g. "pods.backoff"
    Status    Status
    Resource  string            // e.g. "pod/foo in ns/bar"
    Message   string
    Detail    map[string]string // structured fields (image, restartCount, ...)
    Duration  time.Duration
}

type Report struct {
    GeneratedAt time.Time
    Cluster     string
    Findings    []Finding
}
```

Exit-code contract (also documented in
[ADR-0006](adr/0006-output-formats.md)):

| Worst status in report | Exit code |
|------------------------|-----------|
| `OK` only              | 0         |
| `WARN` highest         | 0 (default) or 2 with `--strict` |
| `CRIT` present         | 1         |
| Tool error             | 3         |

## In-cluster vs out-of-cluster

`internal/kube/mode.go` decides at startup:

1. If `--kubeconfig` is set → out-of-cluster.
2. Else if `KUBERNETES_SERVICE_HOST` is set and a valid token is mounted →
   in-cluster.
3. Else default to out-of-cluster with `~/.kube/config`.

A small set of checks (notably etcd direct probes) are only available
in-cluster, or by *launching* a temporary Job. See
[ADR-0003](adr/0003-in-cluster-and-out-of-cluster.md) and
[ADR-0005](adr/0005-etcd-health-collection.md).

## Test runner data flow

```
HealthTest YAML
      │
      ▼
 decode.go ──► validated *HealthTestSpec
      │
      ▼
 runner.Run(spec):
   1. setup.apply         (dynamic client, server-side apply)
   2. wait for readiness  (per-resource conditions, timeout)
   3. for each step:
        - assertion engine handles it (http / exec / log / dns / k8sObject)
        - records Finding
        - on failure, optionally short-circuit
   4. cleanup             (always, unless --keep on failure)
      │
      ▼
 Report
```

## Concurrency

- The check registry runs checks in a worker pool (default 8). Each check is
  expected to be context-cancellable and idempotent (read-only).
- The test runner is sequential within a single `HealthTest` (steps are
  ordered) but multiple `HealthTest` files passed to `khealth test run` can run
  in parallel, gated by `--parallel`.

## Logging

Structured logs via `log/slog`. Log level is `--log-level={error,warn,info,debug}`.
The renderer's stdout is reserved for the report; logs go to stderr.
