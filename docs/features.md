# Feature catalog

The catalog below is the *target* set of checks and capabilities for `khealth`.
Each row maps to a check ID (used in `--checks` / `--skip-checks`) and the
delivery phase from [`roadmap.md`](roadmap.md).

## Workload health (`khealth check pods` / `check workloads`)

| Check ID                 | What it flags                                                | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| `pods.backoff`           | `CrashLoopBackOff`, `ImagePullBackOff`, `ErrImagePull`        | 1 |
| `pods.pending`           | Pods stuck `Pending` past a threshold (default 5m)            | 1 |
| `pods.notReady`          | Containers not ready past readiness threshold                 | 1 |
| `pods.oomKilled`         | Last termination reason `OOMKilled`                           | 1 |
| `pods.restartingHot`     | Restart count above threshold in last hour                    | 2 |
| `pods.evicted`           | Recently evicted pods                                         | 2 |
| `deployments.rollout`    | Rollout stalled, available < desired                          | 1 |
| `daemonsets.rollout`     | `numberReady < desiredNumberScheduled`                        | 1 |
| `statefulsets.rollout`   | `readyReplicas < replicas`                                    | 1 |
| `jobs.failed`            | Jobs with `Failed` condition in the last N hours              | 2 |
| `cronjobs.missed`        | `lastScheduleTime` older than schedule + grace                | 2 |

## Node & infra health (`khealth check nodes`)

| Check ID                 | What it flags                                                | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| `nodes.ready`            | `Ready=False/Unknown`                                         | 1 |
| `nodes.pressure`         | Memory / Disk / PID pressure                                  | 1 |
| `nodes.unschedulable`    | Cordoned nodes (informational unless `--strict`)              | 1 |
| `nodes.kubeletVersion`   | Kubelet version skew vs control plane                         | 2 |
| `nodes.allocatable`      | Allocatable < requested → scheduling pressure                 | 2 |

## Storage & networking

| Check ID                 | What it flags                                                | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| `pvc.pending`            | PVCs not `Bound` past threshold                               | 1 |
| `pv.released`            | Orphaned PVs in `Released` / `Failed`                         | 2 |
| `services.noEndpoints`   | Services with `Endpoints` but zero ready addresses            | 1 |
| `ingress.noBackends`     | Ingress with all backends pointing to empty Endpoints         | 2 |
| `dns.resolution`         | Synthetic in-cluster lookup of `kubernetes.default.svc`       | 2 |

## Control plane (`khealth check controlplane`)

| Check ID                 | What it flags                                                | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| `apiserver.healthz`      | `/readyz` and `/livez` against discovered apiserver           | 1 |
| `apiserver.latency`      | p99 of recent API requests via metrics endpoint               | 3 |
| `scheduler.healthz`      | `/healthz` of `kube-scheduler` (where reachable)              | 2 |
| `controllerMgr.healthz`  | `/healthz` of `kube-controller-manager`                       | 2 |
| `coredns.replicas`       | CoreDNS deployment availability                               | 1 |
| `etcd.health`            | Member list, alarm list, leader presence                      | 2 |
| `etcd.size`              | DB size & in-use size vs `--quota-backend-bytes`              | 2 |
| `etcd.defrag`            | Suggests defrag if fragmentation > threshold                  | 3 |

## Events & policy

| Check ID                 | What it flags                                                | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| `events.warnings`        | `Warning`-level events in the last N minutes                  | 1 |
| `quotas.exhaustion`      | ResourceQuotas at >90% used                                   | 2 |
| `limitRanges.violations` | Pods violating namespace `LimitRange`                         | 3 |
| `psa.warnings`           | Pod Security Admission warnings emitted recently              | 3 |

## Declarative tests (`khealth test`)

| Capability               | Description                                                  | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| `apply` / `delete`       | Server-side apply manifests, automatic cleanup                | 1 |
| `wait` conditions        | Wait for Ready / Available / custom JSONPath                  | 1 |
| `http` assertion         | In-cluster HTTP probe via ephemeral pod                       | 1 |
| `exec` assertion         | `kubectl exec`-style with stdout / exit-code matchers         | 1 |
| `dns` assertion          | DNS lookup from inside the cluster                            | 2 |
| `log` assertion          | Tail logs and assert presence/absence of patterns             | 2 |
| `k8sObject` assertion    | Assert fields on a live object (JSONPath / CEL)               | 2 |
| Parallel tests           | `khealth test run -p 4 ./tests/`                              | 2 |
| Test profiles            | `quick`, `full`, `post-deploy`, `pre-upgrade`                 | 3 |

## Output / integration

| Format                   | Use case                                                     | Phase |
|--------------------------|--------------------------------------------------------------|-------|
| Human table (default)    | Operator at a terminal                                        | 1 |
| `--output json`          | `jq`, dashboards                                              | 1 |
| `--output yaml`          | Pipelines that diff over time                                 | 1 |
| `--output junit`         | CI surfacing as test results                                  | 2 |
| `--output prom`          | `node_exporter` textfile_collector style                      | 2 |
| `--webhook url`          | POST report JSON to Slack / generic webhook                   | 3 |
