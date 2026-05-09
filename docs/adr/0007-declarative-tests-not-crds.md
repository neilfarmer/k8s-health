# ADR-0007: Declarative tests as local YAML, not CRDs

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

Kubernetes operators love CRDs. The natural shape for "declarative health
tests" looks like a CRD: a YAML manifest with `apiVersion`, `kind`, `spec`,
applied to the cluster, reconciled by a controller.

But there are real downsides for *this* tool:

- A CRD requires installing the CRD object before any test can run. That's
  ceremony for the "I just want to smoke-test this fresh cluster" use case.
- A CRD implies a controller, which means a long-running process to maintain,
  RBAC to grant, and a release cadence for the controller separate from the
  CLI.
- Most users describe the tests they want as *one-shot* validations after a
  deploy, in CI, or against a freshly-built cluster — not continuous.

## Decision

The `HealthTest` schema looks CRD-shaped (`apiVersion: khealth.io/v1alpha1`,
`kind: HealthTest`) so it is familiar, but it is **parsed locally by the CLI**
and never applied to the cluster. The cluster only sees the resources the
test's `setup` block applies and the runner's ephemeral helper pods.

`khealth test lint` validates the schema offline. `khealth test run` executes
it.

If real demand emerges later, we can add an *optional* operator that watches
a real CRD and invokes the same `internal/testrunner` package on a schedule.
This stays additive.

## Consequences

**Positive**
- Works on any cluster from minute zero — no install step.
- Lint and CI run anywhere, including with no kube context.
- The runner stays a CLI; we do not have to ship and version a controller.

**Negative**
- The schema looks like a CRD but isn't, which can confuse users who try
  `kubectl apply -f healthtest.yaml`. We mitigate with an explicit error
  message in the `kind` registration documentation and a `khealth test lint`
  hint when a HealthTest is found among manifests.

**Forecloses (for now)**
- Cluster-resident scheduling. Out of scope until we see the demand.
