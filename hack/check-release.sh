#!/usr/bin/env bash
set -euo pipefail

dir=${1:-dist/release}

test -f "$dir/release.yaml"
test -f "$dir/images.txt"
for group in 00-crds 10-kube-vip 20-fabric 60-cert-manager 70-monitoring 90-bedrock; do
  test -d "$dir/manifests/$group"
done
grep -q 'kind: CustomResourceDefinition' "$dir/manifests/00-crds/cert-manager-crds.yaml"
grep -q 'kind: DaemonSet' "$dir/manifests/10-kube-vip/kube-vip.yaml"
grep -q 'name: fabric' "$dir/manifests/20-fabric/fabric.yaml"
grep -q 'BEDROCK_MASTER_IPS' "$dir/manifests/20-fabric/fabric.yaml"
grep -q 'kind: CustomResourceDefinition' "$dir/manifests/00-crds/fabric-crds.yaml"
grep -q 'name: cert-manager' "$dir/manifests/60-cert-manager/cert-manager.yaml"
grep -q 'metrics-server' "$dir/manifests/70-monitoring/monitoring.yaml"
grep -q 'name: bedrock-operator' "$dir/manifests/90-bedrock/bedrock.yaml"
grep -q 'kind: ValidatingAdmissionPolicy' "$dir/manifests/90-bedrock/bedrock.yaml"
! grep -rq 'ghcr.io/cloudyfolks-labs/bedrock:dev' "$dir/manifests" || test "$(grep '^version:' "$dir/release.yaml")" = "version: dev"
echo "check-release passed"
