package release

import (
	"context"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	VarMasterIPs = "BEDROCK_MASTER_IPS"
	VarVIP       = "BEDROCK_VIP"
)

func Vars(ctx context.Context, c client.Client, vip string) (map[string]string, error) {
	var nodes corev1.NodeList
	if err := c.List(ctx, &nodes, client.MatchingLabels{"fabric/role": "master"}); err != nil {
		return nil, err
	}
	var ips []string
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				ips = append(ips, addr.Address)
				break
			}
		}
	}
	sort.Strings(ips)
	vars := map[string]string{VarVIP: vip}
	if len(ips) > 0 {
		vars[VarMasterIPs] = strings.Join(ips, ",")
	}
	return vars, nil
}
