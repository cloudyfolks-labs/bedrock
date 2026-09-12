package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SettingSpec struct {
	// +optional
	Value string `json:"value,omitempty"`
}

type SettingStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Default string `json:"default,omitempty"`
	// +optional
	Applied string `json:"applied,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Value",type=string,JSONPath=`.spec.value`
// +kubebuilder:printcolumn:name="Default",type=string,JSONPath=`.status.default`
type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SettingSpec   `json:"spec,omitempty"`
	Status            SettingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type SettingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Setting `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Setting{}, &SettingList{})
}
