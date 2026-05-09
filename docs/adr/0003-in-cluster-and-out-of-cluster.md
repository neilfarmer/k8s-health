# ADR-0003: Support both in-cluster and out-of-cluster execution

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

The two main consumers of `khealth` have different access patterns:

- **Operators / CI** run from outside the cluster with a kubeconfig.
- **Cluster-local probes** (e.g. a CronJob) run from inside, with a mounted
  ServiceAccount. Some checks (etcd direct probes, in-cluster DNS) can only
  be done from inside.

If we only support out-of-cluster, we lose the ability to ship `khealth` as
a `Job` for periodic health snapshots, and we lose direct etcd access. If we
only support in-cluster, every operator has to first deploy something just to
run a check.

## Decision

`khealth` detects its environment at startup (`internal/kube/mode.go`):

1. `--launch-mode` flag wins if set.
2. Else: `KUBERNETES_SERVICE_HOST` set + valid SA token mounted →
   in-cluster.
3. Else: out-of-cluster with `--kubeconfig` / `$KUBECONFIG` / `~/.kube/config`.

Checks declare their requirements via tags:

```go
// illustrative
type Check interface {
    ID() string
    Run(ctx context.Context, env Env) []Finding
    Requires() Capabilities    // e.g. CapInCluster, CapEtcdCerts, CapAPIServerProxy
}
```

When a required capability is missing, the check emits a `SKIP` finding with a
clear reason (e.g. "etcd.size requires `--launch-mode in-cluster` or
`--etcd-endpoints`"). It does not fail silently and does not hard-fail the
report.

## Consequences

**Positive**
- A single binary works in both contexts — no separate `khealth-incluster`.
- `SKIP`-with-reason makes it obvious *why* a check didn't run.
- Operators can opt in to extra fidelity by passing the right flags.

**Negative**
- More plumbing in each check (capability declarations).
- Two RBAC profiles to document and maintain (out-of-cluster typically uses
  cluster-admin or a read-only role; in-cluster uses a tighter SA we'll ship
  manifests for).

**Forecloses**
- We do *not* support running entirely without API server access. Even the
  declarative test runner needs the API.
