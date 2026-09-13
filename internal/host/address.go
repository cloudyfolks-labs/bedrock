package host

import "context"

func EnsureAddress(ctx context.Context, e Exec, ip, iface string) error {
	_, err := e.Run(ctx, "ip", "addr", "replace", ip+"/32", "dev", iface)
	return err
}

func RemoveAddress(ctx context.Context, e Exec, ip, iface string) error {
	_, err := e.Run(ctx, "ip", "addr", "del", ip+"/32", "dev", iface)
	return err
}
