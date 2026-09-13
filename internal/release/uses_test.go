package release

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestUsesDetectsPlaceholderInBundle(t *testing.T) {
	bundle := Bundle{Groups: []Group{{Name: "fabric", Objects: []*unstructured.Unstructured{objWithEnv("${BEDROCK_MASTER_IPS}")}}}}
	if !Uses(bundle, VarMasterIPs) {
		t.Fatal("expected bundle to use BEDROCK_MASTER_IPS")
	}
}

func TestUsesReportsFalseWithoutPlaceholder(t *testing.T) {
	bundle := Bundle{Groups: []Group{{Name: "fabric", Objects: []*unstructured.Unstructured{objWithEnv("static")}}}}
	if Uses(bundle, VarMasterIPs) {
		t.Fatal("expected bundle not to use BEDROCK_MASTER_IPS")
	}
}

func TestUsesIgnoresOtherPlaceholderNames(t *testing.T) {
	bundle := Bundle{Groups: []Group{{Name: "fabric", Objects: []*unstructured.Unstructured{objWithEnv("${BEDROCK_VIP}")}}}}
	if Uses(bundle, VarMasterIPs) {
		t.Fatal("expected bundle not to use BEDROCK_MASTER_IPS")
	}
}
