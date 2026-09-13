#!/usr/bin/env bash
set -euo pipefail

cluster=bedrock-e2e
image=ghcr.io/cloudyfolks-labs/bedrock:dev

cleanup() {
  local status=$?
  if [ "$status" -ne 0 ]; then
    kubectl -n bedrock-system logs deploy/bedrock-operator --tail=200 || true
    kubectl get setting,host -o yaml || true
  fi
  kind delete cluster --name "$cluster" >/dev/null 2>&1 || true
}
trap cleanup EXIT

kind delete cluster --name "$cluster" >/dev/null 2>&1 || true
kind create cluster --name "$cluster" --wait 120s
docker build -t "$image" -f Containerfile .
kind load docker-image "$image" --name "$cluster"

kubectl apply -f manifests/00-crds
kubectl apply -f manifests/90-bedrock
kubectl -n bedrock-system rollout status deployment/bedrock-operator --timeout=120s

wait_for_resource() {
  local resource=$1
  local timeout=$2
  local waited=0
  until kubectl get "$resource" >/dev/null 2>&1; do
    waited=$((waited + 1))
    if [ "$waited" -ge "$timeout" ]; then
      return 1
    fi
    sleep 1
  done
}

wait_for_resource setting/platform.tls-mode 60
kubectl wait --for=jsonpath='{.status.default}'=SelfSigned setting/platform.tls-mode --timeout=60s

node=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl apply -f - <<EOF2
apiVersion: bedrock.cloudyfolks.io/v1alpha1
kind: Host
metadata:
  name: ${node}
spec:
  roles: [control-plane, ceph-osd, workload]
EOF2
kubectl wait --for=condition=Ready "host/${node}" --timeout=60s
kubectl get node "$node" -o jsonpath='{.metadata.labels}' | grep -q 'role-ceph-osd'

echo "e2e-kind passed"
