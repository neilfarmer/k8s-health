# k8s-health (`khealth`)

> **Status:** Planning. No implementation yet — this repo currently contains design docs, ADRs, and usage examples to scope the tool before any Go code is written.

`khealth` is a single-binary Golang CLI for inspecting and validating the health
of a Kubernetes cluster. It is designed to be useful in three contexts:

1. **Out-of-cluster** — an operator runs `khealth` from a workstation or CI
   runner against a `kubeconfig` (e.g. `khealth check cluster`).
2. **In-cluster** — `khealth` runs as a `Job`, `CronJob`, or sidecar inside the
   cluster, using its `ServiceAccount` to scrape the API and (optionally) the
   control plane.
3. **Synthetic / declarative testing** — a YAML manifest describes resources to
   apply and assertions to run against them; `khealth` deploys, tests, reports,
   and tears down.

## Why another tool?

There are great point tools (`kubectl`, `kube-score`, `popeye`, `k9s`,
`etcdctl`, `sonobuoy`) but each owns a slice of the problem:

| Need                                              | Existing tooling           | Gap `khealth` fills                               |
|---------------------------------------------------|----------------------------|---------------------------------------------------|
| "Show me everything unhealthy right now"          | `kubectl get` + grep       | One opinionated, multi-namespace report           |
| "Is etcd healthy and how big is it?"              | `etcdctl` from a control-plane host | In-cluster job that reports the same thing  |
| "Run a smoke test against my cluster after deploy"| Bash + `kubectl exec`      | Declarative YAML test spec with assertions       |
| "Emit cluster health to Prometheus / CI / Slack"  | Custom scripts             | Built-in JSON / JUnit / Prom / table renderers   |

`khealth` is a **multi-tool**: a swiss-army knife of cluster checks, control-plane
probes, and declarative tests, with a consistent output and exit-code contract.

## Documentation map

| Doc | Purpose |
|-----|---------|
| [`docs/architecture.md`](docs/architecture.md) | Components, data flow, packages |
| [`docs/features.md`](docs/features.md) | Catalog of checks the tool will ship |
| [`docs/cli-reference.md`](docs/cli-reference.md) | Commands, flags, and example invocations |
| [`docs/declarative-tests.md`](docs/declarative-tests.md) | `HealthTest` YAML spec |
| [`docs/roadmap.md`](docs/roadmap.md) | Phased delivery plan |
| [`docs/adr/`](docs/adr/) | Architecture Decision Records |
| [`examples/`](examples/) | Sample `HealthTest` manifests and config |

## At-a-glance

```sh
# Out-of-cluster: full report against current kube context
khealth check cluster

# Just the noisy bits
khealth check pods --all-namespaces --only-unhealthy

# Control plane (uses an in-cluster Job to reach etcd)
khealth check etcd --launch-mode in-cluster

# Run a declarative smoke test
khealth test run ./examples/healthtests/smoke-dns.yaml

# Emit JUnit for CI
khealth check cluster --output junit > report.xml
```

See [`docs/cli-reference.md`](docs/cli-reference.md) for the full command tree.

## Non-goals (for now)

- Replacing `kubectl`. `khealth` reads, asserts, and reports — it is not a
  general resource editor.
- Full chaos / fault-injection. Declarative tests are smoke + integration;
  chaos is out of scope until a later phase.
- Long-running daemon. `khealth` runs to completion, then exits. The
  Prometheus output mode is for `--once`-style scrapes via `textfile_collector`,
  not a persistent server (see [ADR-0006](docs/adr/0006-output-formats.md)).
