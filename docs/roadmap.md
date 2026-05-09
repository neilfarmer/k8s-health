# Roadmap

Phased delivery so each phase produces a *useful* binary, not a half-built one.

## Phase 1 — "Is anything obviously broken?" (MVP)

Goal: a single binary an operator can run against any cluster and get a
trustworthy red/yellow/green list of findings.

- `khealth check cluster | pods | nodes`
- Checks: `pods.backoff`, `pods.pending`, `pods.notReady`, `pods.oomKilled`,
  `deployments.rollout`, `daemonsets.rollout`, `statefulsets.rollout`,
  `nodes.ready`, `nodes.pressure`, `pvc.pending`, `services.noEndpoints`,
  `events.warnings`, `apiserver.healthz`, `coredns.replicas`
- Output: `table`, `json`, `yaml`
- Exit-code contract (see [`architecture.md`](architecture.md))
- `--launch-mode auto` (in/out cluster detection)
- Minimal `khealth test run` for `apply` + `waitFor` + `http` + `exec` steps

Exit criteria: dogfooded on at least one staging cluster + one prod cluster,
zero false positives in a one-week soak.

## Phase 2 — Control plane and CI integration

- `khealth check controlplane`, `khealth check etcd` (all three modes from
  [ADR-0005](adr/0005-etcd-health-collection.md))
- Phase-2 checks from [`features.md`](features.md)
- Output: `junit`, `prom` (textfile_collector format)
- Test runner: `dns`, `log`, `k8sObject` step kinds; `--parallel`,
  `--fail-fast`, `--keep-on-failure`
- `khealth config` with file-backed defaults

## Phase 3 — Polish, ecosystem, profiles

- Phase-3 checks (defrag, latency, PSA, LimitRange)
- Test profiles (`quick`, `full`, `post-deploy`, `pre-upgrade`)
- `--webhook` (Slack-compatible block kit + generic JSON POST)
- Optional `HealthTest` CRD + controller (out of scope unless demand exists)

## Explicit non-goals

- Mutating the cluster outside of test setup/cleanup.
- Replacing `kubectl`, `helm`, or `argocd`.
- Long-running daemon mode. Prom output is for one-shot scrapes.
- Chaos engineering / fault injection.
