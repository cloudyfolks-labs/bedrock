package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
	"github.com/cloudyfolks-labs/bedrock/internal/host"
	"github.com/cloudyfolks-labs/bedrock/internal/hostconfig"
	"github.com/cloudyfolks-labs/bedrock/internal/maintenance"
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

func createHostConfig(t *testing.T, name string, spec v1alpha1.HostConfigSpec) {
	t.Helper()
	hc := &v1alpha1.HostConfig{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: spec}
	if err := k8sClient.Create(context.Background(), hc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { k8sClient.Delete(context.Background(), hc) })
}

func enableManagement(t *testing.T, name string) {
	t.Helper()
	h := getHost(t, name)
	h.Spec.Management.Enabled = true
	if err := k8sClient.Update(context.Background(), &h); err != nil {
		t.Fatal(err)
	}
}

func requireCondition(t *testing.T, name, conditionType string) metav1.Condition {
	t.Helper()
	h := getHost(t, name)
	cond := apimeta.FindStatusCondition(h.Status.Conditions, conditionType)
	if cond == nil {
		t.Fatalf("no %s condition in %+v", conditionType, h.Status.Conditions)
	}
	return *cond
}

func backdateConditions(t *testing.T, name string, at time.Time) {
	t.Helper()
	h := getHost(t, name)
	for i := range h.Status.Conditions {
		h.Status.Conditions[i].LastTransitionTime = metav1.NewTime(at)
	}
	if err := k8sClient.Status().Update(context.Background(), &h); err != nil {
		t.Fatal(err)
	}
}

func TestTickKeepsTransitionTimeUntilStatusChanges(t *testing.T) {
	createHost(t, "node-stable", false, "")
	exec := &host.FakeExec{Responses: map[string]string{"sysctl --system": ""}}
	deps := newDeps(exec, time.Now())
	deps.Node = "node-stable"
	deps.Root = t.TempDir()
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	backdated := time.Now().Add(-time.Hour).Truncate(time.Second).UTC()
	backdateConditions(t, "node-stable", backdated)
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	cond := requireCondition(t, "node-stable", v1alpha1.ConditionManagementApplied)
	if cond.Reason != "ManagementDisabled" || !cond.LastTransitionTime.Time.Equal(backdated) {
		t.Fatalf("transition time must survive an unchanged condition: %+v", cond)
	}
	enableManagement(t, "node-stable")
	createHostConfig(t, "node-stable", v1alpha1.HostConfigSpec{})
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	cond = requireCondition(t, "node-stable", v1alpha1.ConditionManagementApplied)
	if cond.Status != metav1.ConditionTrue || cond.LastTransitionTime.Time.Equal(backdated) {
		t.Fatalf("a status flip must move the transition time: %+v", cond)
	}
}

func TestTickKeepsRebootPendingOutsideWindow(t *testing.T) {
	createHost(t, "node-carry", true, "Sat 02:00-05:00")
	root := t.TempDir()
	exec := &host.FakeExec{Responses: map[string]string{"apt-get update": "", "unattended-upgrade -v": ""}}
	deps := newDeps(exec, time.Date(2026, time.September, 12, 3, 0, 0, 0, time.Local))
	deps.Node = "node-carry"
	deps.Root = root
	deps.Packages = pkgmgr.Manager{Exec: exec, Family: "apt", Root: root}
	if err := pkgmgr.TouchSentinel(root); err != nil {
		t.Fatal(err)
	}
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	cond := requireCondition(t, "node-carry", v1alpha1.ConditionRebootPending)
	if cond.Status != metav1.ConditionTrue || cond.Reason != "SecurityUpdate" {
		t.Fatalf("condition %+v", cond)
	}
	calls := len(exec.Calls)
	monday := time.Date(2026, time.September, 14, 3, 0, 0, 0, time.Local)
	deps.Now = func() time.Time { return monday }
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	if len(exec.Calls) != calls {
		t.Fatalf("a closed window must run nothing: %v", exec.Calls)
	}
	cond = requireCondition(t, "node-carry", v1alpha1.ConditionRebootPending)
	if cond.Status != metav1.ConditionTrue || cond.Reason != "SecurityUpdate" {
		t.Fatalf("a pending reboot must survive a closed window: %+v", cond)
	}
}

func TestTickReportsInvalidWindow(t *testing.T) {
	window := "Someday 02:00-05:00"
	createHost(t, "node-badwindow", true, window)
	exec := &host.FakeExec{}
	deps := newDeps(exec, time.Now())
	deps.Node = "node-badwindow"
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	if len(exec.Calls) != 0 {
		t.Fatalf("calls %v", exec.Calls)
	}
	_, parseErr := maintenance.Parse(window)
	if parseErr == nil {
		t.Fatal("the window must not parse")
	}
	cond := requireCondition(t, "node-badwindow", v1alpha1.ConditionRebootPending)
	if cond.Status != metav1.ConditionFalse || cond.Reason != "InvalidWindow" || cond.Message != parseErr.Error() {
		t.Fatalf("condition %+v", cond)
	}
}

func TestTickReportsFirstFailedStep(t *testing.T) {
	createHost(t, "node-failing", true, "")
	createHostConfig(t, "node-failing", v1alpha1.HostConfigSpec{Modules: []string{"dummy"}})
	exec := &host.FakeExec{
		Responses: map[string]string{"sysctl --system": "", "modprobe dummy": ""},
		Errors:    map[string]error{"modprobe dummy": errors.New("module not found")},
	}
	deps := newDeps(exec, time.Now())
	deps.Node = "node-failing"
	deps.Root = t.TempDir()
	if err := Tick(context.Background(), k8sClient, deps); err != nil {
		t.Fatal(err)
	}
	cond := requireCondition(t, "node-failing", v1alpha1.ConditionManagementApplied)
	if cond.Status != metav1.ConditionFalse || cond.Reason != "StepFailed" || cond.Message != "modules: module not found" {
		t.Fatalf("condition %+v", cond)
	}
}

func TestTickReturnsInventoryErrorAndStillWritesStatus(t *testing.T) {
	createHost(t, "node-noinv", false, "")
	gather := errors.New("gather failed")
	deps := newDeps(&host.FakeExec{}, time.Now())
	deps.Node = "node-noinv"
	deps.Inventory = func(context.Context, host.Exec, string) (v1alpha1.Inventory, error) {
		return v1alpha1.Inventory{}, gather
	}
	if err := Tick(context.Background(), k8sClient, deps); !errors.Is(err, gather) {
		t.Fatalf("err %v", err)
	}
	cond := requireCondition(t, "node-noinv", v1alpha1.ConditionManagementApplied)
	if cond.Status != metav1.ConditionFalse || cond.Reason != "ManagementDisabled" {
		t.Fatalf("condition %+v", cond)
	}
}
