package operator

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
	"github.com/cloudyfolks-labs/bedrock/internal/release"
)

type ClusterReconciler struct {
	Client       client.Client
	Bundle       release.Bundle
	Gates        release.Gates
	Interval     time.Duration
	GroupTimeout time.Duration
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != v1alpha1.ClusterName {
		return ctrl.Result{}, nil
	}
	if err := r.ensureRelease(ctx); err != nil {
		return ctrl.Result{}, err
	}
	var cluster v1alpha1.Cluster
	if err := r.Client.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	embedded := r.Bundle.Spec.Version
	if cluster.Spec.DesiredVersion != embedded {
		return ctrl.Result{}, r.writeStatus(ctx, func(s *v1alpha1.ClusterStatus) {
			setCondition(s, v1alpha1.ConditionProgressing, metav1.ConditionFalse, "AwaitingUpgrade", fmt.Sprintf("desired %s, embedded %s", cluster.Spec.DesiredVersion, embedded), cluster.Generation)
		})
	}
	if cluster.Status.Version == embedded && cluster.Status.Phase != v1alpha1.PhaseFailed {
		return ctrl.Result{}, r.writeStatus(ctx, func(s *v1alpha1.ClusterStatus) {
			s.Phase = v1alpha1.PhaseIdle
			setCondition(s, v1alpha1.ConditionAvailable, metav1.ConditionTrue, "Installed", "", cluster.Generation)
			setCondition(s, v1alpha1.ConditionProgressing, metav1.ConditionFalse, "Installed", "", cluster.Generation)
		})
	}
	if cluster.Status.Phase == v1alpha1.PhaseFailed && cluster.Spec.Upgrade.Action != v1alpha1.UpgradeActionResume {
		return ctrl.Result{}, nil
	}
	if cluster.Spec.Upgrade.Action == v1alpha1.UpgradeActionResume {
		if err := r.clearAction(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, r.install(ctx, cluster.Generation)
}

func (r *ClusterReconciler) install(ctx context.Context, generation int64) error {
	if err := r.writeStatus(ctx, func(s *v1alpha1.ClusterStatus) {
		s.Phase = v1alpha1.PhaseComponents
		s.Components = nil
		setCondition(s, v1alpha1.ConditionProgressing, metav1.ConditionTrue, "Installing", "", generation)
	}); err != nil {
		return err
	}
	report := func(group release.Group, groupErr error) {
		if err := r.writeStatus(ctx, func(s *v1alpha1.ClusterStatus) {
			s.Components = append(s.Components, componentStatus(r.Bundle, group, groupErr))
		}); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "write component status", "component", group.Name)
		}
	}
	err := release.Install(ctx, r.Client, r.Bundle, r.Gates, r.Interval, r.GroupTimeout, report)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		return r.writeStatus(ctx, func(s *v1alpha1.ClusterStatus) {
			s.Phase = v1alpha1.PhaseFailed
			setCondition(s, v1alpha1.ConditionDegraded, metav1.ConditionTrue, "GroupFailed", err.Error(), generation)
			setCondition(s, v1alpha1.ConditionProgressing, metav1.ConditionFalse, "GroupFailed", "", generation)
		})
	}
	return r.writeStatus(ctx, func(s *v1alpha1.ClusterStatus) {
		s.Version = r.Bundle.Spec.Version
		s.Phase = v1alpha1.PhaseIdle
		setCondition(s, v1alpha1.ConditionAvailable, metav1.ConditionTrue, "Installed", "", generation)
		setCondition(s, v1alpha1.ConditionProgressing, metav1.ConditionFalse, "Installed", "", generation)
		setCondition(s, v1alpha1.ConditionDegraded, metav1.ConditionFalse, "Installed", "", generation)
	})
}

func componentStatus(bundle release.Bundle, group release.Group, err error) v1alpha1.ComponentStatus {
	version := bundle.Spec.Version
	for _, component := range bundle.Spec.Components {
		if component.Name == group.Name && component.Version != "" {
			version = component.Version
		}
	}
	status := v1alpha1.ComponentStatus{Name: group.Name, Version: version, Available: err == nil, Degraded: err != nil}
	if err != nil {
		status.Message = err.Error()
	}
	return status
}

func (r *ClusterReconciler) ensureRelease(ctx context.Context) error {
	name := r.Bundle.Spec.Version
	var existing v1alpha1.Release
	err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &existing)
	if errors.IsNotFound(err) {
		created := &v1alpha1.Release{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{v1alpha1.LabelKind: "Release", v1alpha1.LabelName: name}}, Spec: r.Bundle.Spec}
		if err := r.Client.Create(ctx, created); err != nil {
			return err
		}
		existing = *created
	} else if err != nil {
		return err
	}
	next := existing.Status
	next.ObservedGeneration = existing.Generation
	if equality.Semantic.DeepEqual(existing.Spec, r.Bundle.Spec) {
		v1alpha1.SetCondition(&next.Conditions, metav1.Condition{Type: v1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Embedded", ObservedGeneration: existing.Generation})
	} else {
		v1alpha1.SetCondition(&next.Conditions, metav1.Condition{Type: v1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "SpecMismatch", Message: "release object differs from the embedded release", ObservedGeneration: existing.Generation})
	}
	if equality.Semantic.DeepEqual(existing.Status, next) {
		return nil
	}
	existing.Status = next
	return r.Client.Status().Update(ctx, &existing)
}

func (r *ClusterReconciler) clearAction(ctx context.Context, cluster *v1alpha1.Cluster) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Spec.Upgrade.Action = ""
	return r.Client.Patch(ctx, cluster, patch)
}

var statusWriteBackoff = wait.Backoff{Steps: 10, Duration: 20 * time.Millisecond, Factor: 1.5, Jitter: 0.1}

func (r *ClusterReconciler) writeStatus(ctx context.Context, mutate func(*v1alpha1.ClusterStatus)) error {
	return retry.OnError(statusWriteBackoff, errors.IsConflict, func() error {
		var current v1alpha1.Cluster
		if err := r.Client.Get(ctx, client.ObjectKey{Name: v1alpha1.ClusterName}, &current); err != nil {
			return client.IgnoreNotFound(err)
		}
		next := *current.Status.DeepCopy()
		next.ObservedGeneration = current.Generation
		mutate(&next)
		if equality.Semantic.DeepEqual(current.Status, next) {
			return nil
		}
		current.Status = next
		return r.Client.Status().Update(ctx, &current)
	})
}

func setCondition(status *v1alpha1.ClusterStatus, conditionType string, value metav1.ConditionStatus, reason, message string, generation int64) {
	v1alpha1.SetCondition(&status.Conditions, metav1.Condition{Type: conditionType, Status: value, Reason: reason, Message: message, ObservedGeneration: generation})
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Cluster{}).Complete(r)
}
