# ADR-0005: etcd health — three collection modes

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

etcd is the most operationally important component to monitor and also the
hardest to reach:

- It listens on **mutual-TLS** with client certs that live on the control-plane
  hosts (`/etc/kubernetes/pki/etcd/`).
- Its peer endpoints are usually **not exposed** outside the control-plane
  network.
- On managed Kubernetes (EKS, GKE, AKS), the operator has **no direct access**
  to etcd at all.

Different users will be in different boats:

| User                                        | Available access                       |
|---------------------------------------------|----------------------------------------|
| Self-managed K8s, control-plane SSH         | Direct gRPC with certs                 |
| Self-managed K8s, no SSH but cluster-admin  | Schedule a Job on a control-plane node |
| Managed K8s (EKS/GKE/AKS)                   | Only what the API server exposes       |

A one-mode-only design either excludes managed clusters or excludes the
high-fidelity data (db size, fragmentation) that makes the check valuable.

## Decision

Support three modes for `khealth check etcd`:

### 1. Direct gRPC (highest fidelity)

```sh
khealth check etcd \
  --etcd-endpoints https://etcd-0:2379,... \
  --etcd-cacert ... --etcd-cert ... --etcd-key ...
```

Uses `go.etcd.io/etcd/client/v3` to call `Maintenance.Status`,
`Cluster.MemberList`, `Maintenance.AlarmList`. Reports member health, leader,
db-size, db-size-in-use, alarms.

### 2. In-cluster Job (good fidelity, no SSH)

```sh
khealth check etcd --launch-mode in-cluster
```

`khealth` schedules a short-lived `Job` in `kube-system`:
- Tolerates control-plane taints, with `nodeSelector` matching control-plane.
- HostPath-mounts `/etc/kubernetes/pki/etcd` (or a Secret if configured).
- Runs `khealth etcd-probe` (a hidden subcommand) and writes a JSON result to
  a ConfigMap or pipes it back via the Job's pod logs.
- The launching `khealth` waits for completion, reads the result, deletes
  the Job.

### 3. API-server delegated (lowest fidelity, always works)

```sh
khealth check etcd --via-apiserver
```

Calls `kubectl get --raw=/readyz?verbose` (and `/livez?verbose`) and parses
the `etcd` and `etcd-readiness` lines. No size info, but confirms the API
server still considers etcd healthy. This is the only mode that works on
managed Kubernetes.

`auto` mode tries direct → in-cluster Job → apiserver and reports which mode
won.

## Consequences

**Positive**
- Useful answer in every realistic environment.
- The mode used is always reported in the finding's `detail.mode` so the
  consumer knows what fidelity they got.

**Negative**
- Three code paths to maintain. Mitigation: a single `EtcdProbeResult` struct
  with optional fields; the renderers don't care which mode populated it.
- The in-cluster Job mode requires elevated RBAC (create Jobs in
  kube-system, read Secrets) and a tolerable security review.

**Forecloses**
- Continuous etcd monitoring. `khealth` is one-shot; for streaming, use
  `etcd_exporter` and Prometheus. We document this in the help text.
