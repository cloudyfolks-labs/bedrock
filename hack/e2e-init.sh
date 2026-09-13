#!/usr/bin/env bash
set -euo pipefail

version=${VERSION:-dev}
image=${IMAGE:-ghcr.io/cloudyfolks-labs/bedrock:$version}
workdir=$(mktemp -d)
export KUBECONFIG=/var/lib/k0s/pki/admin.conf

dump() {
  local status=$?
  if [ "$status" -ne 0 ]; then
    echo "--- k0s status"
    /usr/local/bin/k0s status || true
    echo "--- nodes"
    kubectl get nodes -o wide || true
    echo "--- pods"
    kubectl get pods -A -o wide || true
    echo "--- cluster"
    kubectl get cluster,release,host,setting -o yaml || true
    echo "--- operator log"
    kubectl -n bedrock-system logs deploy/bedrock-operator --tail=300 || true
    echo "--- fabric logs"
    for pod in $(kubectl -n kube-system get pods -o name 2>/dev/null | grep -E 'fabric|ovn|ovs' || true); do
      echo "--- $pod"; kubectl -n kube-system logs "$pod" --tail=100 --all-containers || true
    done
    journalctl -u k0scontroller --no-pager -n 200 || true
  fi
  rm -rf "$workdir"
}
trap dump EXIT

test "$(id -u)" -eq 0

if ! modinfo openvswitch >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq "linux-modules-extra-$(uname -r)"
fi
modprobe openvswitch
modprobe geneve

iface=$(ip -json route show default | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["dev"])')
nodeip=$(ip -json -4 addr show dev "$iface" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["addr_info"][0]["local"])')
vip=$(echo "$nodeip" | awk -F. '{printf "%s.%s.%s.250", $1, $2, $3}')
if [ "$vip" = "$nodeip" ]; then vip=$(echo "$nodeip" | awk -F. '{printf "%s.%s.%s.251", $1, $2, $3}'); fi

truncate -s 20G "$workdir/osd.img"
device=$(losetup --find --show "$workdir/osd.img")

mkdir -p "$workdir/preload"
docker save "$image" -o "$workdir/preload/bedrock.tar"

VERSION=$version VIP=$vip IFACE=$iface DEVICE=$device envsubst < hack/e2e/cluster.yaml.tmpl > "$workdir/cluster.yaml"
cat "$workdir/cluster.yaml"

bin/bedrock init -f "$workdir/cluster.yaml" --release-dir dist/release --images-dir "$workdir/preload" --timeout 40m

kubectl get nodes -o wide
kubectl wait --for=condition=Ready node --all --timeout=300s
ip -4 addr show dev "$iface" | grep -q "$vip"
kubectl -n kube-system rollout status daemonset/kube-vip --timeout=120s
kubectl get cluster cluster -o jsonpath='{.status.version}' | grep -qx "$version"
kubectl wait --for=condition=Available cluster/cluster --timeout=120s
kubectl -n cert-manager rollout status deployment/cert-manager --timeout=300s
kubectl get host "$(hostname | tr '[:upper:]' '[:lower:]')" -o jsonpath='{.spec.roles}' | grep -q ceph-osd
kubectl get setting storage.replicas -o jsonpath='{.spec.value}' | grep -qx 1
token=$(bin/bedrock token create --roles workload --expiry 10m)
test -n "$token"
echo "$token" | base64 -d 2>/dev/null | grep -q '"k0sToken"' || echo "$token" | python3 -c 'import base64,json,sys; t=sys.stdin.read().strip(); t+="="*(-len(t)%4); json.loads(base64.urlsafe_b64decode(t))["k0sToken"]'
echo "e2e-init passed"
