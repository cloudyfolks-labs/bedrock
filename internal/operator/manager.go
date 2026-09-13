package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
	"github.com/cloudyfolks-labs/bedrock/internal/settings"
)

func Scheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func Run(ctx context.Context, cfg *rest.Config, scheme *runtime.Scheme) error {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, LeaderElection: true, LeaderElectionID: "bedrock-operator", LeaderElectionNamespace: "bedrock-system"})
	if err != nil {
		return err
	}
	if err := (&SettingReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		return err
	}
	if err := (&HostReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		return err
	}
	if err := mgr.Add(seeder{client: mgr.GetClient()}); err != nil {
		return err
	}
	return mgr.Start(ctx)
}

type seeder struct {
	client client.Client
}

func (s seeder) Start(ctx context.Context) error {
	return settings.Seed(ctx, s.client)
}

func (s seeder) NeedLeaderElection() bool {
	return true
}
