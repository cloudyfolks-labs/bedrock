package release

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMultusDaemonHasNoMemoryLimit(t *testing.T) {
	objects, err := loadObjects(os.DirFS(filepath.Join("..", "..")), "manifests/25-multus", "multus")
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range objects {
		if obj.GetKind() != "DaemonSet" || obj.GetName() != "kube-multus-ds" {
			continue
		}
		containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
		for _, item := range containers {
			container, _ := item.(map[string]any)
			if container["name"] != "kube-multus" {
				continue
			}
			if limit, found, _ := unstructured.NestedString(container, "resources", "limits", "memory"); found {
				t.Fatalf("kube-multus runs delegate CNI plugins inside its own memory cgroup, so a fixed limit (%s) gets it OOM killed", limit)
			}
			return
		}
	}
	t.Fatal("container kube-multus of DaemonSet kube-multus-ds not found")
}
