#!/usr/bin/env bash
set -euo pipefail

dir=${1:-dist/release}

test -f "$dir/release.yaml"
test -f "$dir/images.txt"
for group in 00-crds 10-kube-vip 20-fabric 25-multus 30-nmstate 60-cert-manager 70-monitoring 80-kured 90-bedrock; do
  test -d "$dir/manifests/$group"
done
grep -q 'kind: CustomResourceDefinition' "$dir/manifests/00-crds/cert-manager-crds.yaml"
grep -q 'kind: DaemonSet' "$dir/manifests/10-kube-vip/kube-vip.yaml"
grep -q 'name: fabric' "$dir/manifests/20-fabric/fabric.yaml"
grep -q 'BEDROCK_MASTER_IPS' "$dir/manifests/20-fabric/fabric.yaml"
grep -q 'kind: CustomResourceDefinition' "$dir/manifests/00-crds/fabric-crds.yaml"
grep -q 'kind: DaemonSet' "$dir/manifests/25-multus/multus.yaml"
grep -q 'multus-cni:v4.3.1-thick' "$dir/manifests/25-multus/multus.yaml"
grep -q 'name: network-attachment-definitions.k8s.cni.cncf.io' "$dir/manifests/00-crds/multus-crds.yaml"
! grep -q 'kind: CustomResourceDefinition' "$dir/manifests/25-multus/multus.yaml"
grep -q 'name: nmstate-operator' "$dir/manifests/30-nmstate/nmstate.yaml"
grep -q 'kind: NMState' "$dir/manifests/30-nmstate/nmstate.yaml"
grep -q 'name: nmstates.nmstate.io' "$dir/manifests/00-crds/nmstate-crds.yaml"
grep -q 'kubernetes-nmstate-handler:v0.87.0' "$dir/images.txt"
grep -q 'name: cert-manager' "$dir/manifests/60-cert-manager/cert-manager.yaml"
grep -q 'metrics-server' "$dir/manifests/70-monitoring/monitoring.yaml"
grep -q 'kind: DaemonSet' "$dir/manifests/80-kured/kured.yaml"
grep -q 'name: bedrock-operator' "$dir/manifests/90-bedrock/bedrock.yaml"
grep -q 'kind: ValidatingAdmissionPolicy' "$dir/manifests/90-bedrock/bedrock.yaml"
! grep -rq 'ghcr.io/cloudyfolks-labs/bedrock:dev' "$dir/manifests" || test "$(grep '^version:' "$dir/release.yaml")" = "version: dev"
echo "check-release passed"
