# CLI reference (proposed)

Everything below is **design intent**, not implemented yet. The goal is to
nail down the surface area before writing Go.

## Top-level

```
khealth — health checks and declarative tests for Kubernetes clusters

Usage:
  khealth [command]

Available Commands:
  check       Run read-only health checks against a cluster
  test        Run declarative HealthTest manifests
  config      View or write a khealth config file
  version     Print the binary version
  completion  Generate shell completion scripts

Global Flags:
      --kubeconfig string       Path to kubeconfig (default $KUBECONFIG or ~/.kube/config)
      --context string          Kube context to use
  -n, --namespace string        Namespace scope for namespaced checks
  -A, --all-namespaces          Run namespaced checks across all namespaces
      --launch-mode string      out-of-cluster | in-cluster | auto (default "auto")
  -o, --output string           table | json | yaml | junit | prom (default "table")
      --only-unhealthy          Suppress findings with status OK
      --strict                  Treat WARN as failure (exit 2)
      --checks strings          Comma-separated check IDs to include
      --skip-checks strings     Comma-separated check IDs to skip
      --timeout duration        Overall timeout (default 2m)
      --log-level string        error | warn | info | debug (default "info")
      --config string           Path to khealth config file
```

## `khealth check`

```
khealth check cluster                # everything (workloads + nodes + control plane)
khealth check pods                   # workload checks only
khealth check nodes                  # node checks only
khealth check controlplane           # apiserver / scheduler / controller-mgr / etcd / coredns
khealth check etcd                   # just etcd (health + size)
khealth check events                 # warning events in the last window
```

### Examples

```sh
# Quickest possible operator command: "is anything broken right now?"
khealth check cluster --only-unhealthy

# CI smoke check on staging, fail the build on warnings too
khealth check cluster --strict --output junit > khealth.xml

# Pin to specific checks
khealth check cluster --checks pods.backoff,nodes.ready,apiserver.healthz

# Limit blast radius to one namespace
khealth check pods -n payments --only-unhealthy

# Run as a one-shot Job inside the cluster, dump JSON to stdout
khealth check controlplane --launch-mode in-cluster -o json
```

## `khealth check etcd` (special case)

Reading etcd directly requires client certs and a route to the etcd peer
endpoints — usually only reachable from a control-plane node. `khealth`
supports three modes:

```sh
# 1. Direct: certs and endpoints provided by flags or config
khealth check etcd \
  --etcd-endpoints https://etcd-0:2379,https://etcd-1:2379 \
  --etcd-cacert /etc/kubernetes/pki/etcd/ca.crt \
  --etcd-cert   /etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --etcd-key    /etc/kubernetes/pki/etcd/healthcheck-client.key

# 2. In-cluster Job: khealth schedules a privileged Job in kube-system
#    that mounts etcd certs from the host or from a Secret, runs the same
#    binary in `etcd-probe` subcommand, and pipes results back.
khealth check etcd --launch-mode in-cluster

# 3. API-server delegated: read /readyz?verbose which includes etcd readiness.
#    Lower fidelity (no size info) but works without privileges.
khealth check etcd --via-apiserver
```

See [ADR-0005](adr/0005-etcd-health-collection.md) for why all three exist.

## `khealth test`

```
khealth test run    PATH...      # run one or more HealthTest YAMLs
khealth test lint   PATH...      # validate schema without executing
khealth test list   PATH...      # show test names, steps, est. duration
```

### Examples

```sh
# Run a single test file
khealth test run ./examples/healthtests/smoke-dns.yaml

# Run a directory of tests in parallel, abort on first failure
khealth test run ./tests/post-deploy/ --parallel 4 --fail-fast

# Validate-only (no cluster contact except discovery)
khealth test lint ./tests/

# Keep deployed resources around when a test fails (for debugging)
khealth test run ./tests/canary.yaml --keep-on-failure

# Pipe JUnit into a CI surface
khealth test run ./tests/ -o junit > junit.xml
```

## `khealth config`

```sh
khealth config init             # writes ~/.config/khealth/config.yaml
khealth config view             # prints the resolved config
khealth config set defaults.output json
```

A config file lets teams pin defaults so operators don't have to remember
flags. See [`examples/config/khealth.yaml`](../examples/config/khealth.yaml).

## Sample output

### Human table (default)

```
khealth check cluster --only-unhealthy

CLUSTER  staging-eu-west-1
TIME     2026-05-09T09:14:22Z

WORKLOADS
  CRIT  pods.backoff             pod/api-7d9 in ns/payments     CrashLoopBackOff (12 restarts in 4m)
  WARN  deployments.rollout      deployment/web in ns/storefront  3/4 ready, last update 8m ago

NODES
  CRIT  nodes.ready              node/ip-10-0-3-21              Ready=Unknown for 3m

CONTROL PLANE
  WARN  etcd.size                cluster/etcd                    db-size 6.8 GiB (85% of 8 GiB quota)

3 findings (2 CRIT, 1 WARN). Exit code: 1
```

### JSON

```json
{
  "generatedAt": "2026-05-09T09:14:22Z",
  "cluster": "staging-eu-west-1",
  "findings": [
    {
      "check": "pods.backoff",
      "status": "CRIT",
      "resource": "pod/api-7d9 in ns/payments",
      "message": "CrashLoopBackOff (12 restarts in 4m)",
      "detail": { "image": "registry/api:1.42", "exitCode": "137" }
    }
  ]
}
```
