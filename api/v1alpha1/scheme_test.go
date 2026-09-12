package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestSchemeRegistersSetting(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	gvk := GroupVersion.WithKind("Setting")
	if !scheme.Recognizes(gvk) {
		t.Fatalf("scheme does not recognize %s", gvk)
	}
	if GroupVersion.Group != "bedrock.cloudyfolks.io" || GroupVersion.Version != "v1alpha1" {
		t.Fatalf("group version %s", GroupVersion)
	}
}
