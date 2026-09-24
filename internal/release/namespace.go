package release

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var clusterScopedKinds = map[schema.GroupKind]struct{}{
	{Group: "", Kind: "ComponentStatus"}:                                              {},
	{Group: "", Kind: "Namespace"}:                                                    {},
	{Group: "", Kind: "Node"}:                                                         {},
	{Group: "", Kind: "PersistentVolume"}:                                             {},
	{Group: "admissionregistration.k8s.io", Kind: "MutatingAdmissionPolicy"}:          {},
	{Group: "admissionregistration.k8s.io", Kind: "MutatingAdmissionPolicyBinding"}:   {},
	{Group: "admissionregistration.k8s.io", Kind: "MutatingWebhookConfiguration"}:     {},
	{Group: "admissionregistration.k8s.io", Kind: "ValidatingAdmissionPolicy"}:        {},
	{Group: "admissionregistration.k8s.io", Kind: "ValidatingAdmissionPolicyBinding"}: {},
	{Group: "admissionregistration.k8s.io", Kind: "ValidatingWebhookConfiguration"}:   {},
	{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:                 {},
	{Group: "apiregistration.k8s.io", Kind: "APIService"}:                             {},
	{Group: "authentication.k8s.io", Kind: "SelfSubjectReview"}:                       {},
	{Group: "authentication.k8s.io", Kind: "TokenReview"}:                             {},
	{Group: "authorization.k8s.io", Kind: "SelfSubjectAccessReview"}:                  {},
	{Group: "authorization.k8s.io", Kind: "SelfSubjectRulesReview"}:                   {},
	{Group: "authorization.k8s.io", Kind: "SubjectAccessReview"}:                      {},
	{Group: "certificates.k8s.io", Kind: "CertificateSigningRequest"}:                 {},
	{Group: "flowcontrol.apiserver.k8s.io", Kind: "FlowSchema"}:                       {},
	{Group: "flowcontrol.apiserver.k8s.io", Kind: "PriorityLevelConfiguration"}:       {},
	{Group: "networking.k8s.io", Kind: "IPAddress"}:                                   {},
	{Group: "networking.k8s.io", Kind: "IngressClass"}:                                {},
	{Group: "networking.k8s.io", Kind: "ServiceCIDR"}:                                 {},
	{Group: "node.k8s.io", Kind: "RuntimeClass"}:                                      {},
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}:                         {},
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"}:                  {},
	{Group: "resource.k8s.io", Kind: "DeviceClass"}:                                   {},
	{Group: "resource.k8s.io", Kind: "ResourceSlice"}:                                 {},
	{Group: "scheduling.k8s.io", Kind: "PriorityClass"}:                               {},
	{Group: "storage.k8s.io", Kind: "CSIDriver"}:                                      {},
	{Group: "storage.k8s.io", Kind: "CSINode"}:                                        {},
	{Group: "storage.k8s.io", Kind: "StorageClass"}:                                   {},
	{Group: "storage.k8s.io", Kind: "VolumeAttachment"}:                               {},
	{Group: "storage.k8s.io", Kind: "VolumeAttributesClass"}:                          {},
}

func defaultNamespaces(objects []*unstructured.Unstructured, namespace string) ([]*unstructured.Unstructured, error) {
	scopes := crdScopes(objects)
	out := make([]*unstructured.Unstructured, 0, len(objects))
	for _, obj := range objects {
		if obj.GetNamespace() != "" {
			out = append(out, obj)
			continue
		}
		scoped, err := namespaced(obj.GroupVersionKind(), scopes)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
		if !scoped {
			out = append(out, obj)
			continue
		}
		placed := obj.DeepCopy()
		placed.SetNamespace(namespace)
		out = append(out, placed)
	}
	return out, nil
}

func crdScopes(objects []*unstructured.Unstructured) map[schema.GroupKind]string {
	scopes := map[schema.GroupKind]string{}
	for _, obj := range objects {
		if obj.GetKind() != "CustomResourceDefinition" {
			continue
		}
		group, _, _ := unstructured.NestedString(obj.Object, "spec", "group")
		kind, _, _ := unstructured.NestedString(obj.Object, "spec", "names", "kind")
		scope, _, _ := unstructured.NestedString(obj.Object, "spec", "scope")
		scopes[schema.GroupKind{Group: group, Kind: kind}] = scope
	}
	return scopes
}

func namespaced(gvk schema.GroupVersionKind, scopes map[schema.GroupKind]string) (bool, error) {
	if _, ok := clusterScopedKinds[gvk.GroupKind()]; ok {
		return false, nil
	}
	if scope, ok := scopes[gvk.GroupKind()]; ok {
		return scope == "Namespaced", nil
	}
	if clientgoscheme.Scheme.Recognizes(gvk) {
		return true, nil
	}
	return false, fmt.Errorf("the scope of %s is unknown", gvk.GroupKind())
}
