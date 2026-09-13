package release

import (
	"os"
	"testing"
)

func TestLoadGood(t *testing.T) {
	bundle, err := Load(os.DirFS("testdata/good"))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Spec.Version != "v0.1.0-test" || bundle.Spec.K0sVersion != "v1.36.3+k0s.0" {
		t.Fatalf("spec %+v", bundle.Spec)
	}
	if len(bundle.Images) != 1 {
		t.Fatalf("images %v", bundle.Images)
	}
	if len(bundle.Groups) != 2 || bundle.Groups[0].Name != "crds" || bundle.Groups[0].Order != 0 || bundle.Groups[1].Name != "bedrock" || bundle.Groups[1].Order != 90 {
		t.Fatalf("groups %+v", bundle.Groups)
	}
	if len(bundle.Groups[1].Objects) != 3 {
		t.Fatalf("objects %d", len(bundle.Groups[1].Objects))
	}
	for _, obj := range bundle.Groups[1].Objects {
		if obj.GetLabels()[ComponentLabel] != "bedrock" {
			t.Fatalf("label missing on %s/%s", obj.GetKind(), obj.GetName())
		}
	}
	if bundle.Groups[0].Objects[0].GetKind() != "CustomResourceDefinition" {
		t.Fatalf("kind %s", bundle.Groups[0].Objects[0].GetKind())
	}
}

func TestLoadRejectsBadGroupName(t *testing.T) {
	if _, err := Load(os.DirFS("testdata/badname")); err == nil {
		t.Fatal("expected error for group directory without numeric prefix")
	}
}

func TestLoadRejectsMissingReleaseFile(t *testing.T) {
	if _, err := Load(os.DirFS("testdata/good/manifests")); err == nil {
		t.Fatal("expected error when release.yaml is missing")
	}
}
