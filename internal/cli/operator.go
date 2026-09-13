package cli

import (
	"fmt"
	"io"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/cloudyfolks-labs/bedrock/internal/operator"
)

func operatorCommand(_ []string, _, stderr io.Writer) int {
	ctrl.SetLogger(zap.New())
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	scheme, err := operator.Scheme()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := operator.Run(ctrl.SetupSignalHandler(), cfg, scheme); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
