package host

import (
	"fmt"
	"os"
	"path/filepath"
)

const containerdDropInDir = "etc/k0s/containerd.d"

func MirrorFiles(mirror string) map[string]string {
	return map[string]string{
		"cri-registry.toml":           "[plugins.\"io.containerd.cri.v1.images\".registry]\nconfig_path = \"/etc/k0s/containerd.d/certs.d\"\n",
		"certs.d/_default/hosts.toml": fmt.Sprintf("[host.%q]\ncapabilities = [\"pull\", \"resolve\"]\n", mirror),
	}
}

func EnsureMirror(root, mirror string) error {
	for rel, content := range MirrorFiles(mirror) {
		path := filepath.Join(root, containerdDropInDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
