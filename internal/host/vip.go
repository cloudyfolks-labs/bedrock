package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const vipUnitTemplate = `[Unit]
Description=Bedrock API VIP
Before=k0scontroller.service
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'ping -c 1 -W 1 %s >/dev/null 2>&1 || ip addr replace %s/32 dev %s'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`

func VIPUnit(vip, iface string) string {
	return fmt.Sprintf(vipUnitTemplate, vip, vip, iface)
}

func EnsureVIPUnit(ctx context.Context, e Exec, root, vip, iface string) error {
	path := filepath.Join(root, "etc", "systemd", "system", "bedrock-vip.service")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(VIPUnit(vip, iface)), 0o644); err != nil {
		return err
	}
	if _, err := e.Run(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err := e.Run(ctx, "systemctl", "enable", "bedrock-vip.service")
	return err
}
