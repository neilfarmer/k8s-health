# ADR-0010: Node-probe via ephemeral Pod

- **Status**: Proposed
- **Date**: 2026-05-09

## Context

A growing class of checks needs information that is not exposed by the
Kubernetes API: filesystem usage on the node root + container runtime
disks, kernel sysctls (e.g. `vm.max_map_count`, `fs.inotify.max_user_*`),
kubelet flags, container-runtime version + state (containerd, cri-o),
process counts, NTP / clock skew, and so on.

Three patterns considered:

1. **Permanent DaemonSet** ("node-probe-ds"). One Pod per node, listens on
   a host port, khealth queries it. Very capable, but adds operational
   surface — install/upgrade/RBAC/version skew, plus a permanently
   privileged Pod on every node. Most users don't want another agent
   purely for one-shot health runs.

2. **Ephemeral Job per check run**. khealth schedules a privileged Pod
   on each node (or a sample) for the duration of one invocation, reads
   what it needs, deletes the Pod. Same pattern we already use for
   `etcd-probe` (ADR-0005). No persistent footprint; explicit per-run
   cost (~5–15s).

3. **`kubectl debug node/<name>` style**. The API server already supports
   debug Pods via the v1.25+ "node debugger". Same idea as #2 but uses
   the upstream API instead of rolling our own Job. Requires
   `pods.create` + `pods/exec` on a privileged namespace.

## Decision

Adopt option **2**: an ephemeral Pod per node per run, modelled on the
existing `etcd-probe` Job (ADR-0005).

- New subcommand `khealth node-probe` (mirror of `etcd-probe`) runs
  inside the Pod, collects the data, prints one JSON line on stdout, and
  exits. The launcher reads the log to recover the result.
- Launcher (in `internal/nodeprobe`) creates one Pod per target node
  (default: every Ready node; opt-in sampling via `--node-probe-sample N`),
  with `hostNetwork=true`, `hostPID=true`, hostPath mounts of `/`,
  `/proc`, `/sys`, and `nodeName` pinning. Pods carry the same labels as
  the etcd-probe Job (`app.kubernetes.io/name=khealth`,
  `component=node-probe`).
- Concurrency: bounded worker pool (default `min(nodeCount, 8)`).
  Per-Pod timeout: 30s. Cleanup is best-effort via owner refs +
  `TTLSecondsAfterFinished`.
- Image: same as `--etcd-job-image` (or a dedicated `--node-job-image`
  if users want to lock it down separately). `auto` mode reuses the
  etcd image when set.
- New capability `CapNodeProbe` mirrors `CapInCluster`: checks that
  need node-level data declare it via `Requires()`. The runner gates
  these on `--node-probe=auto|on|off`. Default `auto` enables the probe
  iff a job image is configured.
- New CLI flags:
  - `--node-probe auto|on|off` (default `auto`).
  - `--node-probe-image <ref>` (defaults to `--etcd-job-image`).
  - `--node-probe-namespace <ns>` (default `kube-system`).
  - `--node-probe-sample N` (default 0 = all Ready nodes).

## Consequences

**Positive**
- Reuses an established pattern (ADR-0005); no new operational story.
- No persistent agent. Users who never run node-probe pay zero cost.
- Easy to gate per-distro (RKE2/k3s/kubeadm differ on kubelet flag
  paths) — checks declare `Distros()` (ADR-0009) plus `CapNodeProbe`.
- Each check stays one file in `internal/checks/<id>.go` (ADR-0004).

**Negative**
- Privileged Pods + hostPath mounts require `cluster-admin`-equivalent
  RBAC. We will document this in `INSTALL.md` and refuse to launch
  with `--node-probe=on` if the SA can't `create pods`.
- Per-run latency: ~5–15s for image pull + scheduling + collection.
  Mitigated by parallel scheduling and sampling.
- Each invocation creates and deletes Pods, which is visible in
  `kube-system` event history. Acceptable for a health tool.

**Forecloses (for now)**
- Permanent DaemonSet (option 1). If users want continuous metrics we
  push them to node-exporter/Prometheus.
- `kubectl debug node` (option 3). Lower control over scheduling +
  mounts; harder to label Pods consistently.

## Open questions

- Should checks that need node-probe also have a metrics-server-only
  variant for users who refuse privileged Pods? (Likely yes for disk:
  kubelet exposes `node-fs-*` metrics on `/metrics/cadvisor` which is
  pod-proxiable.)
- Do we ship a default image, or always require `--node-probe-image`?
  Defer until the first concrete check is implemented.
