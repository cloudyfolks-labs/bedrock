package release

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVarsFromMasterNodes(t *testing.T) {
	c, _ := startTestEnv(t)
	ctx := context.Background()
	for _, n := range []struct {
		name, ip string
		master   bool
	}{{"b", "10.0.10.12", true}, {"a", "10.0.10.11", true}, {"w", "10.0.10.20", false}} {
		labels := map[string]string{}
		if n.master {
			labels["fabric/role"] = "master"
		}
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: n.name, Labels: labels}}
		if err := c.Create(ctx, node); err != nil {
			t.Fatal(err)
		}
		node.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: n.ip}}
		if err := c.Status().Update(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	vars, err := Vars(ctx, c, "10.0.10.10")
	if err != nil {
		t.Fatal(err)
	}
	if vars[VarMasterIPs] != "10.0.10.11,10.0.10.12" || vars[VarVIP] != "10.0.10.10" {
		t.Fatalf("vars %v", vars)
	}
}

func TestVarsWithoutMasters(t *testing.T) {
	c, _ := startTestEnv(t)
	vars, err := Vars(context.Background(), c, "10.0.10.10")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := vars[VarMasterIPs]; ok {
		t.Fatal("no masters must leave the var absent")
	}
}

func TestWaitVarsWaitsForMasterRegistration(t *testing.T) {
	c, _ := startTestEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "m", Labels: map[string]string{"fabric/role": "master"}}}
		if err := c.Create(context.Background(), node); err != nil {
			return
		}
		node.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.10.11"}}
		_ = c.Status().Update(context.Background(), node)
	}()
	vars, err := WaitVars(ctx, c, "10.0.10.10", 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if vars[VarMasterIPs] != "10.0.10.11" {
		t.Fatalf("vars %v", vars)
	}
}

func TestWaitVarsTimesOut(t *testing.T) {
	c, _ := startTestEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if _, err := WaitVars(ctx, c, "10.0.10.10", 20*time.Millisecond); err == nil {
		t.Fatal("expected a timeout error")
	}
}
