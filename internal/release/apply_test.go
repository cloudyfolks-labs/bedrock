package release

import (
	"context"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestInventoryOf(t *testing.T) {
	bundle, err := Load(os.DirFS("testdata/good"))
	if err != nil {
		t.Fatal(err)
	}
	inv := InventoryOf(bundle.Groups)
	if len(inv) != 4 {
		t.Fatalf("inventory size %d", len(inv))
	}
	if _, ok := inv[ObjectRef{APIVersion: "v1", Kind: "ConfigMap", Namespace: "release-test", Name: "alpha"}]; !ok {
		t.Fatal("alpha missing from inventory")
	}
}

func TestApplyThenPrune(t *testing.T) {
	c, _ := startTestEnv(t)
	ctx := context.Background()
	bundle, err := Load(os.DirFS("testdata/good"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: SystemNamespace}}); err != nil {
		t.Fatal(err)
	}
	applier := Applier{Client: c}
	for _, group := range bundle.Groups {
		if err := applier.Apply(ctx, group); err != nil {
			t.Fatalf("apply %s: %v", group.Name, err)
		}
	}
	var alpha corev1.ConfigMap
	if err := c.Get(ctx, client.ObjectKey{Namespace: "release-test", Name: "alpha"}, &alpha); err != nil {
		t.Fatal(err)
	}
	if alpha.Data["key"] != "one" || alpha.Labels[ComponentLabel] != "bedrock" {
		t.Fatalf("alpha %+v", alpha)
	}
	if len(alpha.ManagedFields) == 0 || alpha.ManagedFields[0].Manager != FieldManager {
		t.Fatalf("field manager %+v", alpha.ManagedFields)
	}

	if err := applier.Apply(ctx, bundle.Groups[1]); err != nil {
		t.Fatalf("second apply must be idempotent: %v", err)
	}

	current := InventoryOf(bundle.Groups)
	if err := WriteInventory(ctx, c, current); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadInventory(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != len(current) {
		t.Fatalf("stored %d current %d", len(stored), len(current))
	}

	next := Inventory{}
	for ref := range current {
		if ref.Name != "beta" {
			next[ref] = struct{}{}
		}
	}
	if err := applier.Prune(ctx, current, next); err != nil {
		t.Fatal(err)
	}
	var beta corev1.ConfigMap
	if err := c.Get(ctx, client.ObjectKey{Namespace: "release-test", Name: "beta"}, &beta); !errors.IsNotFound(err) {
		t.Fatalf("beta must be pruned, got %v", err)
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "release-test", Name: "alpha"}, &alpha); err != nil {
		t.Fatalf("alpha must survive: %v", err)
	}

	empty := Inventory{}
	if err := applier.Prune(ctx, current, empty); err != nil {
		t.Fatal(err)
	}
	var ns corev1.Namespace
	if err := c.Get(ctx, client.ObjectKey{Name: "release-test"}, &ns); err != nil {
		t.Fatalf("namespace must never be pruned: %v", err)
	}
}

func TestReadInventoryWhenMissing(t *testing.T) {
	c, _ := startTestEnv(t)
	ctx := context.Background()
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: SystemNamespace}}); err != nil {
		t.Fatal(err)
	}
	inv, err := ReadInventory(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) != 0 {
		t.Fatalf("expected empty inventory, got %d", len(inv))
	}
}
