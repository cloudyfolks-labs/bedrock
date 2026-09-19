package host

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
)

const containerdDropInDir = "etc/k0s/containerd.d"

func MirrorFilesFor(mirrors []v1alpha1.MirrorSpec) map[string]string {
	files := map[string]string{"cri-registry.toml": "[plugins.\"io.containerd.cri.v1.images\".registry]\nconfig_path = \"/etc/k0s/containerd.d/certs.d\"\n"}
	for _, mirror := range mirrors {
		files["certs.d/"+mirror.Registry+"/hosts.toml"] = fmt.Sprintf("[host.%q]\ncapabilities = [\"pull\", \"resolve\"]\n", mirror.Endpoint)
	}
	return files
}

func MirrorFiles(mirror string) map[string]string {
	return MirrorFilesFor([]v1alpha1.MirrorSpec{{Registry: "_default", Endpoint: mirror}})
}

func EnsureMirrors(root string, mirrors []v1alpha1.MirrorSpec) error {
	return writeFiles(filepath.Join(root, containerdDropInDir), MirrorFilesFor(mirrors))
}

func EnsureMirror(root, mirror string) error {
	return EnsureMirrors(root, []v1alpha1.MirrorSpec{{Registry: "_default", Endpoint: mirror}})
}

func writeFiles(dir string, files map[string]string) error {
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
