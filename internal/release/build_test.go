package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildRendersChartsAndDirs(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Fatal("helm is required on PATH for the release builder tests")
	}
	cfg, err := LoadBuildConfig(filepath.Join("testdata", "build", "components.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	opts := BuildOptions{Version: "v9.9.9", Image: "ghcr.io/cloudyfolks-labs/bedrock:v9.9.9", Out: out, Helm: "helm", Root: filepath.Join("testdata", "build")}
	if err := Build(cfg, opts); err != nil {
		t.Fatal(err)
	}
	bundle, err := Load(os.DirFS(out))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Spec.Version != "v9.9.9" || bundle.Spec.Image != opts.Image || bundle.Spec.K0sVersion != "v1.36.3+k0s.0" {
		t.Fatalf("spec %+v", bundle.Spec)
	}
	if len(bundle.Groups) != 3 || bundle.Groups[0].Name != "crds" || bundle.Groups[1].Name != "widgets" || bundle.Groups[2].Name != "bedrock" {
		t.Fatalf("groups %+v", bundle.Groups)
	}
	if len(bundle.Groups[0].Objects) != 1 || bundle.Groups[0].Objects[0].GetKind() != "CustomResourceDefinition" {
		t.Fatalf("crds group %+v", bundle.Groups[0].Objects)
	}
	widgets := bundle.Groups[1].Objects
	if len(widgets) != 2 || widgets[0].GetKind() != "Namespace" || widgets[0].GetName() != "widgets" || widgets[1].GetKind() != "Deployment" {
		t.Fatalf("widgets group %+v", widgets)
	}
	replicas, _, _ := unstructuredInt(widgets[1], "spec", "replicas")
	if replicas != 2 {
		t.Fatalf("values not applied, replicas %d", replicas)
	}
	operatorImages := ImagesOf([]Group{bundle.Groups[2]})
	if len(operatorImages) != 1 || operatorImages[0] != opts.Image {
		t.Fatalf("operator image not rewritten: %v", operatorImages)
	}
	images := bundle.Images
	if len(images) != 2 || images[0] != "ghcr.io/cloudyfolks-labs/bedrock:v9.9.9" || images[1] != "quay.io/example/widgets:0.1.0" {
		t.Fatalf("images %v", images)
	}
	if len(bundle.Spec.Components) != 3 || bundle.Spec.Components[1].Version != "0.1.0" || bundle.Spec.Components[1].Image != "quay.io/example/widgets:0.1.0" {
		t.Fatalf("components %+v", bundle.Spec.Components)
	}
}

func unstructuredInt(obj *unstructured.Unstructured, fields ...string) (int64, bool, error) {
	return unstructured.NestedInt64(obj.Object, fields...)
}

func TestBuildMergesSameNamedOutputs(t *testing.T) {
	cfg, err := LoadBuildConfig(filepath.Join("testdata", "build", "components-merge.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	opts := BuildOptions{Version: "v9.9.9", Image: "ghcr.io/cloudyfolks-labs/bedrock:v9.9.9", Out: out, Helm: "helm", Root: filepath.Join("testdata", "build")}
	if err := Build(cfg, opts); err != nil {
		t.Fatal(err)
	}
	bundle, err := Load(os.DirFS(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Groups) != 1 || bundle.Groups[0].Name != "bedrock" {
		t.Fatalf("groups %+v", bundle.Groups)
	}
	objects := bundle.Groups[0].Objects
	if len(objects) != 2 {
		t.Fatalf("expected 2 merged objects, got %+v", objects)
	}
	kinds := map[string]bool{}
	for _, obj := range objects {
		kinds[obj.GetKind()] = true
	}
	if !kinds["Deployment"] || !kinds["ConfigMap"] {
		t.Fatalf("expected both Deployment and ConfigMap, got %+v", objects)
	}
}

func TestValidateOutRejectsDangerousPaths(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"", "/", ".", filepath.Join("..", filepath.Base(cwd))}
	for _, path := range paths {
		if err := validateOut(path); err == nil {
			t.Fatalf("validateOut(%q) = nil, want error", path)
		}
	}
	if err := validateOut(t.TempDir()); err != nil {
		t.Fatalf("validateOut(tempdir) = %v, want nil", err)
	}
}
