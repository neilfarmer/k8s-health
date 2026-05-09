#!/usr/bin/env bash
# Spin up the same kind cluster the integration workflow uses, build khealth,
# load the image, and (optionally) seed the cluster with broken resources so
# you can see real findings.
#
# Subcommands:
#   up           Create the cluster + build + load image. Idempotent.
#   down         Delete the cluster.
#   demo         Run `up` then apply examples/demo/broken-resources.yaml,
#                wait for issues to materialize, and print a `khealth check
#                cluster` report against the live cluster.
#   test         Run `make integration` against the running cluster (keeps it).
#   khealth ARGS Pass-through: `./dist/khealth ARGS` with kubeconfig pointed at
#                the kind cluster. Example: ./hack/local-kind.sh khealth check pods -A
#
# Requires: kind, kubectl, docker, go, make.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-khealth-it}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.31.0}"
KIND_CONFIG="${KIND_CONFIG:-$REPO_ROOT/test/integration/kind-config.yaml}"
KHEALTH_IMAGE="${KHEALTH_IMAGE:-khealth:local}"

cmd="${1:-up}"; shift || true

cluster_exists() { kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; }

ensure_up() {
  if ! cluster_exists; then
    echo "==> creating kind cluster $CLUSTER_NAME ($KIND_NODE_IMAGE)"
    kind create cluster \
      --name "$CLUSTER_NAME" \
      --image "$KIND_NODE_IMAGE" \
      --config "$KIND_CONFIG" \
      --wait 120s
  else
    echo "==> kind cluster $CLUSTER_NAME already exists"
  fi
  echo "==> building khealth"
  ( cd "$REPO_ROOT" && make build )
  echo "==> building image $KHEALTH_IMAGE"
  ( cd "$REPO_ROOT" && docker build -t "$KHEALTH_IMAGE" . >/dev/null )
  echo "==> loading $KHEALTH_IMAGE into kind"
  kind load docker-image "$KHEALTH_IMAGE" --name "$CLUSTER_NAME"
  kubectl --context "kind-$CLUSTER_NAME" cluster-info
}

down() {
  if cluster_exists; then
    echo "==> deleting cluster $CLUSTER_NAME"
    kind delete cluster --name "$CLUSTER_NAME"
  else
    echo "==> no cluster $CLUSTER_NAME to delete"
  fi
}

run_khealth() {
  KUBECONFIG="$(kind get kubeconfig-path --name "$CLUSTER_NAME" 2>/dev/null \
    || (kind export kubeconfig --name "$CLUSTER_NAME" --kubeconfig /tmp/kc-$CLUSTER_NAME >/dev/null && echo /tmp/kc-$CLUSTER_NAME))" \
    "$REPO_ROOT/dist/khealth" "$@"
}

demo() {
  ensure_up
  echo
  echo "==> seeding broken resources from examples/demo/broken-resources.yaml"
  kubectl --context "kind-$CLUSTER_NAME" apply -f "$REPO_ROOT/examples/demo/broken-resources.yaml"
  echo
  echo "==> waiting 45s for failures to materialize…"
  sleep 45
  echo
  echo "==> khealth check cluster --output table --only-unhealthy"
  echo "----------------------------------------------------------"
  run_khealth check cluster --output table --only-unhealthy || true
  echo
  echo "Cluster left running. Useful follow-ups:"
  echo "  ./hack/local-kind.sh khealth check pods -A"
  echo "  ./hack/local-kind.sh khealth check cluster -o json | jq"
  echo "  kubectl --context kind-$CLUSTER_NAME get pods -n khealth-demo"
  echo "  ./hack/local-kind.sh down            # tear down"
}

run_tests() {
  ensure_up
  echo
  echo "==> running integration tests against $CLUSTER_NAME"
  cd "$REPO_ROOT"
  KUBECONFIG="$HOME/.kube/config" \
    KHEALTH_BIN="$REPO_ROOT/dist/khealth" \
    KHEALTH_IMAGE="$KHEALTH_IMAGE" \
    go test -v -tags=integration -timeout=15m -count=1 ./test/integration/...
}

case "$cmd" in
  up)      ensure_up ;;
  down)    down ;;
  demo)    demo ;;
  test)    run_tests ;;
  khealth) run_khealth "$@" ;;
  *)       echo "unknown subcommand: $cmd"; exit 2 ;;
esac
