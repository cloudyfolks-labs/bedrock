package operator

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
)

func TestHostReconcilerLabelsNode(t *testing.T) {
	c, cfg := StartTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr, err := ctrl.NewManager(cfg, testManagerOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := (&HostReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		t.Fatal(err)
	}
	go func() { _ = mgr.Start(ctx) }()

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"kubernetes.io/hostname": "node-1"}}}
	if err := c.Create(ctx, node); err != nil {
		t.Fatal(err)
	}
	host := &v1alpha1.Host{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: v1alpha1.HostSpec{Roles: []string{v1alpha1.RoleControlPlane, v1alpha1.RoleCephOSD}}}
	if err := c.Create(ctx, host); err != nil {
		t.Fatal(err)
	}

	waitFor(t, func() bool {
		var got corev1.Node
		if err := c.Get(ctx, client.ObjectKey{Name: "node-1"}, &got); err != nil {
			return false
		}
		return got.Labels["bedrock.cloudyfolks.io/role-ceph-osd"] == "true" && len(got.Spec.Taints) == 1
	})
	waitFor(t, func() bool {
		var got v1alpha1.Host
		if err := c.Get(ctx, client.ObjectKey{Name: "node-1"}, &got); err != nil {
			return false
		}
		return v1alpha1.IsConditionTrue(got.Status.Conditions, v1alpha1.ConditionReady) && got.Status.ObservedGeneration == got.Generation
	})

	var current v1alpha1.Host
	if err := c.Get(ctx, client.ObjectKey{Name: "node-1"}, &current); err != nil {
		t.Fatal(err)
	}
	current.Spec.Roles = []string{v1alpha1.RoleWorkload}
	if err := c.Update(ctx, &current); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		var got corev1.Node
		if err := c.Get(ctx, client.ObjectKey{Name: "node-1"}, &got); err != nil {
			return false
		}
		_, stale := got.Labels["bedrock.cloudyfolks.io/role-ceph-osd"]
		return !stale && len(got.Spec.Taints) == 0 && got.Labels["kubernetes.io/hostname"] == "node-1"
	})
}

func TestHostReconcilerWithoutNode(t *testing.T) {
	c, cfg := StartTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr, err := ctrl.NewManager(cfg, testManagerOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := (&HostReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		t.Fatal(err)
	}
	go func() { _ = mgr.Start(ctx) }()

	host := &v1alpha1.Host{ObjectMeta: metav1.ObjectMeta{Name: "ghost"}, Spec: v1alpha1.HostSpec{Roles: []string{v1alpha1.RoleWorkload}}}
	if err := c.Create(ctx, host); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		var got v1alpha1.Host
		if err := c.Get(ctx, client.ObjectKey{Name: "ghost"}, &got); err != nil {
			return false
		}
		return len(got.Status.Conditions) == 1 && got.Status.Conditions[0].Reason == "NodeMissing"
	})
}

func TestHostReconcilerCreatesHostFromNodeLabels(t *testing.T) {
	c, cfg := StartTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr, err := ctrl.NewManager(cfg, testManagerOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := (&HostReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		t.Fatal(err)
	}
	go func() { _ = mgr.Start(ctx) }()

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "joined-1", Labels: map[string]string{"bedrock.cloudyfolks.io/role-workload": "true", "bedrock.cloudyfolks.io/role-ceph-osd": "true"}}}
	if err := c.Create(ctx, node); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		var got v1alpha1.Host
		if err := c.Get(ctx, client.ObjectKey{Name: "joined-1"}, &got); err != nil {
			return false
		}
		return len(got.Spec.Roles) == 2 && got.Spec.Roles[0] == v1alpha1.RoleCephOSD && got.Spec.Roles[1] == v1alpha1.RoleWorkload && got.Labels[v1alpha1.LabelKind] == "Host"
	})

	plain := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "plain-1"}}
	if err := c.Create(ctx, plain); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	var ghost v1alpha1.Host
	if err := c.Get(ctx, client.ObjectKey{Name: "plain-1"}, &ghost); err == nil {
		t.Fatal("node without role labels must not get a Host")
	}
}
