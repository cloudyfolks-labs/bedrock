package agent

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
)

func ApplyStatus(ctx context.Context, c client.Client, name string, status v1alpha1.HostStatus) error {
	desired := v1alpha1.Host{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "Host"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     status,
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&desired)
	if err != nil {
		return err
	}
	delete(content, "spec")
	unstructured.RemoveNestedField(content, "metadata", "creationTimestamp")
	patch := &unstructured.Unstructured{Object: content}
	return c.Status().Patch(ctx, patch, client.Apply, client.FieldOwner(v1alpha1.AgentFieldManager), client.ForceOwnership)
}
