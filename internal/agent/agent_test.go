package agent

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
	"github.com/cloudyfolks-labs/bedrock/internal/host"
	"github.com/cloudyfolks-labs/bedrock/internal/hostconfig"
	"github.com/cloudyfolks-labs/bedrock/internal/pkgmgr"
)

func fakeInventory(ctx context.Context, _ host.Exec, _ string) (v1alpha1.Inventory, error) {
	return v1alpha1.Inventory{CPU: v1alpha1.CPUInfo{Cores: 4}, MemoryBytes: 1 << 30}, nil
}

func newDeps(exec *host.FakeExec, now time.Time) Deps {
	return Deps{Exec: exec, Root: "/nonexistent", Node: "node-a", Now: func() time.Time { return now }, Interval: time.Hour, Inventory: fakeInventory, Apply: hostconfig.Apply, Packages: pkgmgr.Manager{Exec: exec, Family: "apt", Root: "/nonexistent"}}
}

func createHost(t *testing.T, name string, managed bool, window string) {
	t.Helper()
	h := &v1alpha1.Host{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: v1alpha1.HostSpec{Roles: []string{"workload"}, Management: v1alpha1.ManagementSpec{Enabled: managed}, MaintenanceWindow: window}}
	if err := k8sClient.Create(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { k8sClient.Delete(context.Background(), h) })
}

func getHost(t *testing.T, name string) v1alpha1.Host {
	t.Helper()
	var h v1alpha1.Host
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name}, &h); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestTickWritesInventoryOnly(t *testing.T) {
	createHost(t, "node-a", false, "")
	exec := &host.FakeExec{}
	if err := Tick(context.Background(), k8sClient, newDeps(exec, time.Now())); err != nil {
		t.Fatal(err)
	}
	h := getHost(t, "node-a")
	if h.Status.Inventory.CPU.Cores != 4 || h.Status.Inventory.MemoryBytes != 1<<30 {
		t.Fatalf("inventory %+v", h.Status.Inventory)
	}
	if v1alpha1.IsConditionTrue(h.Status.Conditions, v1alpha1.ConditionManagementApplied) {
		t.Fatal("management must not be applied")
	}
	if len(exec.Calls) != 0 {
		t.Fatalf("no host command may run when management is off: %v", exec.Calls)
	}
}

func TestTickAppliesHostConfigWhenManaged(t *testing.T) {
	createHost(t, "node-a", true, "")
	hc := &v1alpha1.HostConfig{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}, Spec: v1alpha1.HostConfigSpec{Sysctls: map[string]string{"net.ipv4.ip_forward": "1"}}}
	if err := k8sClient.Create(context.Background(), hc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { k8sClient.Delete(context.Background(), hc) })
	root := t.TempDir()
	exec := &host.FakeExec{Responses: map[string]string{"sysctl --system": ""}}
	deps := newDeps(exec, time.Now())
	deps.Root = root
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	h := getHost(t, "node-a")
	if !v1alpha1.IsConditionTrue(h.Status.Conditions, v1alpha1.ConditionManagementApplied) {
		t.Fatalf("conditions %+v", h.Status.Conditions)
	}
	if h.Status.Applied.Generation != hc.Generation || len(h.Status.Applied.Steps) != 7 {
		t.Fatalf("applied %+v", h.Status.Applied)
	}
	calls := len(exec.Calls)
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	if len(exec.Calls) != calls {
		t.Fatalf("second tick at the same generation must not re-apply: %v", exec.Calls)
	}
}

func TestTickReportsMissingHostConfig(t *testing.T) {
	createHost(t, "node-a", true, "")
	if err := Tick(context.Background(), k8sClient, newDeps(&host.FakeExec{}, time.Now())); err != nil {
		t.Fatal(err)
	}
	h := getHost(t, "node-a")
	for _, c := range h.Status.Conditions {
		if c.Type == v1alpha1.ConditionManagementApplied && c.Reason == "NoHostConfig" && c.Status == metav1.ConditionFalse {
			return
		}
	}
	t.Fatalf("conditions %+v", h.Status.Conditions)
}

func TestTickRunsSecurityUpdateInWindow(t *testing.T) {
	createHost(t, "node-a", true, "Sat 02:00-05:00")
	root := t.TempDir()
	exec := &host.FakeExec{Responses: map[string]string{"apt-get update": "", "unattended-upgrade -v": ""}}
	saturday := time.Date(2026, time.September, 12, 3, 0, 0, 0, time.Local)
	deps := newDeps(exec, saturday)
	deps.Root = root
	deps.Packages = pkgmgr.Manager{Exec: exec, Family: "apt", Root: root}
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	if len(exec.Calls) != 2 {
		t.Fatalf("calls %v", exec.Calls)
	}
	h := getHost(t, "node-a")
	if v1alpha1.IsConditionTrue(h.Status.Conditions, v1alpha1.ConditionRebootPending) {
		t.Fatal("no reboot pending without the marker file")
	}
	pkgmgr.TouchSentinel(root)
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	h = getHost(t, "node-a")
	if !v1alpha1.IsConditionTrue(h.Status.Conditions, v1alpha1.ConditionRebootPending) {
		t.Fatalf("conditions %+v", h.Status.Conditions)
	}
}

func TestTickSkipsUpdateOutsideWindow(t *testing.T) {
	createHost(t, "node-a", true, "Sat 02:00-05:00")
	exec := &host.FakeExec{}
	monday := time.Date(2026, time.September, 14, 3, 0, 0, 0, time.Local)
	if err := Tick(context.Background(), k8sClient, newDeps(exec, monday)); err != nil {
		t.Fatal(err)
	}
	if len(exec.Calls) != 0 {
		t.Fatalf("calls %v", exec.Calls)
	}
}

func TestApplyStatusKeepsOtherManagersFields(t *testing.T) {
	createHost(t, "node-a", false, "")
	h := getHost(t, "node-a")
	h.Status.KubernetesVersion = "v1.36.3"
	if err := k8sClient.Status().Update(context.Background(), &h); err != nil {
		t.Fatal(err)
	}
	if err := ApplyStatus(context.Background(), k8sClient, "node-a", v1alpha1.HostStatus{Inventory: v1alpha1.Inventory{MemoryBytes: 42}}); err != nil {
		t.Fatal(err)
	}
	h = getHost(t, "node-a")
	if h.Status.KubernetesVersion != "v1.36.3" || h.Status.Inventory.MemoryBytes != 42 {
		t.Fatalf("status %+v", h.Status)
	}
}
