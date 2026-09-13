package roles

import (
	"testing"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
)

func TestLabelsAndFromLabelsRoundTrip(t *testing.T) {
	in := []string{v1alpha1.RoleWorkload, v1alpha1.RoleControlPlane}
	labels := Labels(in)
	if labels[LabelPrefix+"control-plane"] != "true" || labels[LabelPrefix+"workload"] != "true" {
		t.Fatalf("labels %v", labels)
	}
	out := FromLabels(labels)
	if len(out) != 2 || out[0] != v1alpha1.RoleControlPlane || out[1] != v1alpha1.RoleWorkload {
		t.Fatalf("round trip %v", out)
	}
}

func TestFromLabelsIgnoresForeignAndFalse(t *testing.T) {
	out := FromLabels(map[string]string{"kubernetes.io/hostname": "n", LabelPrefix + "ceph-osd": "false", LabelPrefix + "workload": "true"})
	if len(out) != 1 || out[0] != v1alpha1.RoleWorkload {
		t.Fatalf("got %v", out)
	}
}

func TestTaintsWithoutWorkload(t *testing.T) {
	if len(Taints([]string{v1alpha1.RoleControlPlane})) != 1 || len(Taints([]string{v1alpha1.RoleWorkload})) != 0 {
		t.Fatal("taint rule broken")
	}
}
