package release

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestImagesOf(t *testing.T) {
	deploy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "d"},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"initContainers": []any{map[string]any{"name": "i", "image": "quay.io/a/init:1"}},
			"containers":     []any{map[string]any{"name": "c", "image": "ghcr.io/x/y:2"}, map[string]any{"name": "d", "image": "quay.io/a/init:1"}},
		}}},
	}}
	cm := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "c"}, "data": map[string]any{"image": "not-a-container-field-but-collected:3"}}}
	images := ImagesOf([]Group{{Name: "a", Objects: []*unstructured.Unstructured{deploy, cm}}})
	want := []string{"ghcr.io/x/y:2", "not-a-container-field-but-collected:3", "quay.io/a/init:1"}
	if len(images) != len(want) {
		t.Fatalf("images %v", images)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("images %v, want %v", images, want)
		}
	}
}
