package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionReady       = "Ready"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
)

func SetCondition(conditions *[]metav1.Condition, cond metav1.Condition) {
	meta.SetStatusCondition(conditions, cond)
}

func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	return meta.IsStatusConditionTrue(conditions, conditionType)
}
