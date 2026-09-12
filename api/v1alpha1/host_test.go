package v1alpha1

import "testing"

func TestValidRole(t *testing.T) {
	for _, role := range AllRoles() {
		if !ValidRole(role) {
			t.Fatalf("%q must be valid", role)
		}
	}
	if ValidRole("storage") {
		t.Fatal("storage must be invalid")
	}
}

func TestHostHasRole(t *testing.T) {
	host := Host{Spec: HostSpec{Roles: []string{RoleControlPlane, RoleWorkload}}}
	if !host.HasRole(RoleWorkload) {
		t.Fatal("workload expected")
	}
	if host.HasRole(RoleCephOSD) {
		t.Fatal("ceph-osd not expected")
	}
}
