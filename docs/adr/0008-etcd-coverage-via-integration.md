# ADR-0008: `internal/etcd` is covered by integration tests, not units

- **Status**: Accepted
- **Date**: 2026-05-09

## Context

`internal/etcd` implements three modes for reading etcd state per
[ADR-0005](0005-etcd-health-collection.md):

1. **Direct gRPC** — uses `go.etcd.io/etcd/client/v3` to call
   `Maintenance.Status`, `Cluster.MemberList`, and `Maintenance.AlarmList`.
2. **In-cluster Job** — schedules a privileged `Job` in `kube-system` with
   host-mounted etcd certs, runs `khealth etcd-probe`, reads the JSON
   result back from the pod's logs.
3. **via-apiserver** — parses `/readyz?verbose` against the configured
   apiserver.

The first two cannot be unit-tested without a real etcd or a real
Kubernetes cluster. We considered:

- **Embedding etcd in unit tests** (e.g. `embed.Etcd`). Pulls in a large
  test surface and ~2s of startup per test.
- **Spinning up etcd via testcontainers** for unit runs. Adds Docker as a
  unit-test dependency; meaningfully complicates `make test`.
- **Hand-rolled mocks of `clientv3.Client`** and `kubernetes.Interface`.
  Brittle. Tests would assert call shapes, not behavior.

## Decision

Cover `internal/etcd` with **integration tests against kind**, not unit
tests. kind already runs a full kubeadm control plane (apiserver +
scheduler + controller-manager + etcd as static pods on the control-plane
node), and the `integration` GitHub Actions workflow already provisions
one for the existing `pods.backoff` and `nodes.ready` cases.

We exclude `internal/etcd` from the unit-test `-coverpkg` so the 80%
unit-coverage gate isn't penalized by code that's only exercisable against
a real cluster.

The exclusion lives in two places (kept in lockstep):

- `Makefile` — `COVER_PKGS = $(shell go list ./... | grep -v /internal/etcd ...)`
- `.github/workflows/ci.yml` — same shell pipeline in the `test` job

## Consequences

**Positive**
- Honest unit-coverage signal for the rest of the code (still ≥ 80%).
- No new test-time dependency on Docker or embedded etcd.
- Real coverage of the etcd modes happens against the actual K8s + etcd
  versions kind ships with — closer to production than any mock.

**Negative**
- Bugs in `internal/etcd` only surface in CI's slower integration job, not
  in the fast unit run.
- The exclusion can drift if the package gets new files we *could* unit
  test. Mitigation: anything testable should still get a unit test under
  `internal/etcd`; this ADR doesn't say "no unit tests in there", it says
  "don't make the gate measure that package."

**Forecloses**
- None. We can revisit this if a future Go release ships a workable
  in-process etcd test fixture, or if we adopt the merged-coverage
  approach (Go binary coverage in integration → merged with unit profile)
  in a future ADR.

## Alternatives revisited

| Option | Why we didn't pick it |
|--------|----------------------|
| Embedded `embed.Etcd` | Slow test boot, large surface |
| testcontainers | Docker as unit-test dep; complicates `make test` |
| Lower the 80% gate | Penalizes the rest of the code that *is* testable |
| Merged unit+integration coverage (Go 1.20 `-cover` on built binaries → `go tool covdata`) | Real option, but more pipeline plumbing than we want for one package. Reasonable as a future ADR. |
