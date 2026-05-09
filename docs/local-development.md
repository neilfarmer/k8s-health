# Local development against a real cluster

The integration tests in CI run against a [kind][kind] cluster. The same
cluster — with the same K8s version and node config — can run on your
laptop so you can poke at the tool interactively, see real findings, and
tail logs.

[kind]: https://kind.sigs.k8s.io/

## Prerequisites

- `docker` (Linux containerd or Docker Desktop)
- `kind` ≥ v0.24
- `kubectl`
- `go` 1.26+ (toolchain auto-bumps via `go.mod`)
- `make`

## One-shot demo

```sh
make local-demo
```

This will:

1. Build `dist/khealth` and the `khealth:local` container image.
2. Create a 3-node kind cluster (`khealth-it`) using
   `test/integration/kind-config.yaml` — the same config CI uses.
3. Load the container image into the kind cluster.
4. Apply the [broken-resources demo manifest][demo] into a `khealth-demo`
   namespace: a CrashLoopBackOff pod, an `ImagePullBackOff` pod, an empty
   Service, and a stuck Deployment.
5. Wait 45 seconds for failures to show up in the API.
6. Run `khealth check cluster --output table --only-unhealthy` against the
   live cluster and print findings.

The cluster is left running so you can keep iterating.

[demo]: ../examples/demo/broken-resources.yaml

## Iterating

```sh
# fresh report any time
./hack/local-kind.sh khealth check cluster -o table --only-unhealthy

# narrow to one category
./hack/local-kind.sh khealth check pods -A

# JSON for jq pipelines
./hack/local-kind.sh khealth check cluster -o json | jq '.findings[] | select(.status=="CRIT")'

# try the in-cluster mode by deploying khealth as a Job
kubectl --context kind-khealth-it apply -f deploy/in-cluster/rbac.yaml
kubectl --context kind-khealth-it apply -f deploy/in-cluster/job.yaml
kubectl --context kind-khealth-it logs -n khealth job/khealth-check-once -f
```

## Running the integration test suite locally

```sh
make local-test     # uses the running cluster, keeps it
# or, the original one-shot:
make integration    # builds + creates kind + runs tests + tears down
```

`make local-test` is the better loop while developing — the cluster stays
up between runs so you don't pay 60-90s of provisioning for each iteration.

## Cleaning up

```sh
make local-down
```

## What's in `examples/demo/broken-resources.yaml`

| Resource                          | Triggers (eventually)                       |
|-----------------------------------|---------------------------------------------|
| pod/crashloop                     | `pods.backoff` CRIT (CrashLoopBackOff)       |
| pod/badimage                      | `pods.backoff` CRIT (ImagePullBackOff)       |
| pvc/orphan                        | `pvc.pending` WARN after 5m threshold        |
| svc/empty-svc                     | `services.noEndpoints` WARN                  |
| deployment/stuck-rollout (×3 bad) | `deployments.rollout` CRIT                   |

Plus, on the kind cluster itself, `khealth check controlplane` will show
real `apiserver.healthz`, `coredns.replicas`, and (depending on what
pieces of kubeadm kind exposes) `scheduler.healthz` /
`controllerMgr.healthz`.

## Variables

The script honors a few env vars:

| Var               | Default                    | Notes                                         |
|-------------------|----------------------------|-----------------------------------------------|
| `CLUSTER_NAME`    | `khealth-it`               | kind cluster name (matches CI)                |
| `KIND_NODE_IMAGE` | `kindest/node:v1.31.0`     | Pin to a different K8s minor                  |
| `KIND_CONFIG`     | `test/integration/kind-config.yaml` | Topology — defaults to 1 cp + 2 workers |
| `KHEALTH_IMAGE`   | `khealth:local`            | Local image tag for the in-cluster Job        |
