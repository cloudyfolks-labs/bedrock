package operator

import (
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
)

const (
	labelRolePrefix = "bedrock.cloudyfolks.io/role-"
	labelFabricGW   = "fabric.cloudyfolks.io/external-gw"
	labelFabricRole = "fabric/role"
	taintNoWorkload = "bedrock.cloudyfolks.io/no-workload"
)

func managedLabelKeys() []string {
	keys := []string{labelFabricGW, labelFabricRole}
	for _, role := range v1alpha1.AllRoles() {
		keys = append(keys, labelRolePrefix+role)
	}
	return keys
}

func RoleLabels(roles []string) map[string]string {
	labels := map[string]string{}
	for _, role := range roles {
		labels[labelRolePrefix+role] = "true"
	}
	if slices.Contains(roles, v1alpha1.RoleFabricGateway) {
		labels[labelFabricGW] = "true"
		labels[labelFabricRole] = "master"
	}
	return labels
}

func RoleTaints(roles []string) []corev1.Taint {
	if slices.Contains(roles, v1alpha1.RoleWorkload) {
		return nil
	}
	return []corev1.Taint{{Key: taintNoWorkload, Effect: corev1.TaintEffectNoSchedule}}
}

func ApplyRoles(node *corev1.Node, roles []string) bool {
	desiredLabels := maps.Clone(node.Labels)
	if desiredLabels == nil {
		desiredLabels = map[string]string{}
	}
	for _, key := range managedLabelKeys() {
		delete(desiredLabels, key)
	}
	maps.Copy(desiredLabels, RoleLabels(roles))

	desiredTaints := slices.DeleteFunc(slices.Clone(node.Spec.Taints), func(t corev1.Taint) bool { return t.Key == taintNoWorkload })
	desiredTaints = append(desiredTaints, RoleTaints(roles)...)

	changed := !maps.Equal(node.Labels, desiredLabels) || !taintsEqual(node.Spec.Taints, desiredTaints)
	node.Labels = desiredLabels
	node.Spec.Taints = desiredTaints
	return changed
}

type taintKey struct {
	Key    string
	Value  string
	Effect corev1.TaintEffect
}

func taintSet(taints []corev1.Taint) map[taintKey]int {
	set := make(map[taintKey]int, len(taints))
	for _, t := range taints {
		set[taintKey{Key: t.Key, Value: t.Value, Effect: t.Effect}]++
	}
	return set
}

func taintsEqual(a, b []corev1.Taint) bool {
	return maps.Equal(taintSet(a), taintSet(b))
}
