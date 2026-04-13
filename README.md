# k8s-health

[![CI](https://github.com/neilfarmer/k8s-health/actions/workflows/ci.yml/badge.svg)](https://github.com/neilfarmer/k8s-health/actions/workflows/ci.yml)
[![Security](https://github.com/neilfarmer/k8s-health/actions/workflows/security.yml/badge.svg)](https://github.com/neilfarmer/k8s-health/actions/workflows/security.yml)

Kubernetes cluster health diagnostics CLI. Scans your cluster for common failure modes and reports issues in a clean, actionable format.

## What It Checks

| Checker | What it detects |
|---------|-----------------|
| **Pods** | CrashLoopBackOff, ImagePullBackOff, OOMKilled, stuck Pending, container errors |
| **Nodes** | NotReady, DiskPressure, MemoryPressure, PIDPressure, cordoned nodes |
| **Deployments** | Unavailable replicas, stalled rollouts (ProgressDeadlineExceeded) |
| **DaemonSets** | Pods not ready, misscheduled pods |
| **StatefulSets** | Replicas not ready |
| **Jobs** | Failed jobs, exceeded deadlines, suspended jobs |
| **CRDs** | Not established, name conflicts |
| **Helm Releases** | Failed, pending-install, pending-upgrade releases |
| **PVCs** | Pending or lost persistent volume claims |
| **Services** | LoadBalancer without external IP, no ready endpoints |
| **Ingresses** | No address assigned, missing backend services |
| **Events** | Recent warning events grouped by object |

## Installation

### From GitHub Releases

Download the latest binary for your platform from [Releases](https://github.com/neilfarmer/k8s-health/releases).

```bash
# Linux (amd64)
curl -LO https://github.com/neilfarmer/k8s-health/releases/latest/download/k8s-health_linux_amd64.tar.gz
tar xzf k8s-health_linux_amd64.tar.gz
sudo mv k8s-health /usr/local/bin/

# macOS (arm64)
curl -LO https://github.com/neilfarmer/k8s-health/releases/latest/download/k8s-health_darwin_arm64.tar.gz
tar xzf k8s-health_darwin_arm64.tar.gz
sudo mv k8s-health /usr/local/bin/
```

### From Source

```bash
go install github.com/neilfarmer/k8s-health@latest
```

## Usage

```bash
# Scan entire cluster
k8s-health scan

# Scan specific namespaces
k8s-health scan --namespace production,staging

# JSON output for pipelines
k8s-health scan -o json

# Only show critical issues
k8s-health scan --severity critical

# Run specific checkers only
k8s-health scan --checkers pods,nodes,helm

# Exclude specific checkers
k8s-health scan --exclude events,ingresses

# Use a specific kubeconfig/context
k8s-health scan --kubeconfig ~/.kube/prod-config --context prod-cluster

# Disable colors (auto-detected for non-TTY)
k8s-health scan --no-color

# Show passing checks too
k8s-health scan --verbose
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed, cluster is healthy |
| `1` | Issues found at or above minimum severity |
| `2` | Tool error (can't connect, invalid args) |

### Pipeline Example

```yaml
# GitHub Actions
- name: Check cluster health
  run: |
    k8s-health scan --severity critical -o json > health-report.json
```

## Development

```bash
# Build
make build

# Run tests
make test

# Run with coverage
make coverage

# Lint
make lint

# Run acceptance tests (requires KIND)
kind create cluster --config test/acceptance/kind-config.yaml
kubectl create namespace test-fixtures
kubectl apply -f test/acceptance/fixtures/
sleep 45
make acceptance-test
kind delete cluster
```

## Release

Releases are automated via GoReleaser. To create a new release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers the release workflow, building binaries for Linux, macOS, and Windows (amd64 + arm64).

## License

MIT
