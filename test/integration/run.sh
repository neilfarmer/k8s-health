#!/usr/bin/env bash
# Local equivalent of the integration GitHub Actions job: builds khealth,
# spins up a kind cluster, runs the integration tests, and tears the cluster
# down on exit. Requires kind and kubectl on PATH.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-khealth-it}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.31.0}"

cleanup() {
  if [[ "${KEEP_CLUSTER:-0}" != "1" ]]; then
    kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
  else
    echo "KEEP_CLUSTER=1 — leaving cluster $CLUSTER_NAME running"
  fi
}
trap cleanup EXIT

cd "$REPO_ROOT"

echo "==> building khealth"
make build

if ! kind get clusters | grep -qx "$CLUSTER_NAME"; then
  echo "==> creating kind cluster $CLUSTER_NAME ($KIND_NODE_IMAGE)"
  kind create cluster \
    --name "$CLUSTER_NAME" \
    --image "$KIND_NODE_IMAGE" \
    --config "$REPO_ROOT/test/integration/kind-config.yaml" \
    --wait 120s
fi

kubectl cluster-info --context "kind-$CLUSTER_NAME"
kubectl get nodes -o wide

export KHEALTH_BIN="$REPO_ROOT/dist/khealth"
export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

echo "==> running integration tests"
go test -tags=integration -timeout=15m -count=1 ./test/integration/...
