# ADR-0009: Distro-aware checks

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

Some checks are meaningful only on a specific Kubernetes distribution.
RKE2 ships `helm-install-*` Jobs and `HelmChart` CRs in `kube-system`;
kubeadm ships `kube-controller-manager` static Pods on a known port
binding; EKS hides the control plane entirely. A `coredns.replicas` check
that grep's for a `coredns` Deployment is a SKIP on RKE2 (it's named
`rke2-coredns-rke2-coredns`) but should still run.

We want to:

1. Run a meaningful set of checks on every distro without spurious SKIPs.
2. Add distro-specific checks (RKE2 helm-install jobs, kubeadm static-pod
   manifests, etc.) without polluting the generic check list.
3. Let users override auto-detection when it's wrong (mixed clusters,
   unusual installs).

The candidates considered:

1. **Per-distro packages** (`internal/checks/distro/rke2/...`), each with
   its own registry that the runner merges based on detection. Strong
   isolation, but duplicates the registry plumbing and breaks ADR-0004's
   "one check per file in `internal/checks/`" rule.
2. **`Distros() []Distro` method on `Check`**, files stay flat in
   `internal/checks/`, runner filters by detected/forced distro. Matches
   ADR-0004 with one new method on the interface.
3. **New `Capabilities` bits** (`CapDistroRKE2`, etc.). Overloads a
   bitmask that today means "runtime requirement satisfied" with
   "environment matches" — different concept, would muddy
   `Requires()`.

## Decision

Adopt option 2.

- Add a `Distro` enum: `auto`, `rke2`, `k3s`, `kubeadm`, `eks`. (Other
  managed clouds can be added when there's a check that actually needs
  them.)
- Add an **optional** `DistroAware` interface with `Distros() []Distro`.
  Checks that don't implement it run on every distro — every existing
  check is unchanged. Distro-specific checks implement the method and
  list the distros they apply to.
- Add a global `--distro` flag, default `auto`. `auto` runs a detector
  that reads cluster-visible signals (Leases, namespaces, node
  annotations) once at startup; explicit values skip detection.
- The registry filter gates checks: a check that implements
  `DistroAware` runs iff the detected/forced distro is in the returned
  slice.
- Distro-specific checks live alongside generic ones in
  `internal/checks/<id>.go` — naming convention `rke2_<id>.go` for
  discoverability. Registration is unchanged.

Detection order for `auto`:

1. `rke2` Lease in `kube-system` → RKE2.
2. `k3s` Lease (or `node.kubernetes.io/instance-type=k3s`) → k3s.
3. `eks.amazonaws.com/...` labels on nodes → EKS.
4. Otherwise → kubeadm (the conservative default; the kubeadm-specific
   checks all degrade to SKIP when their inputs are missing).

## Consequences

**Positive**
- One file per check, ADR-0004 still holds.
- Adding a distro is one enum value plus one detector branch.
- Generic checks can declare `Distros() []Distro{}` and never think
  about it again.
- Users can force a distro when detection is wrong: `--distro rke2`.

**Negative**
- One more dimension when filtering checks (`--checks` /
  `--skip-checks` / category / distro). The CLI already exposes the
  first three; a fourth is cheap.
- Distro detection has to stay cheap and read-only — a slow detector
  would slow every invocation.

**Forecloses (for now)**
- Per-distro registries (option 1). If the count of distro-specific
  checks ever exceeds ~20 we may revisit and split into subpackages.
- Per-version gating within a distro (e.g. "RKE2 ≥ 1.30"). Out of
  scope; if needed, a check can inspect server version itself.
