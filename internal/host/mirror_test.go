package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMirrorFiles(t *testing.T) {
	files := MirrorFiles("https://mirror.example.com")
	if !strings.Contains(files["cri-registry.toml"], `config_path = "/etc/k0s/containerd.d/certs.d"`) {
		t.Fatalf("cri-registry.toml %q", files["cri-registry.toml"])
	}
	hosts := files["certs.d/_default/hosts.toml"]
	if !strings.Contains(hosts, `[host."https://mirror.example.com"]`) || !strings.Contains(hosts, `capabilities = ["pull", "resolve"]`) {
		t.Fatalf("hosts.toml %q", hosts)
	}
	if strings.Contains(files["cri-registry.toml"], "version = 2") {
		t.Fatal("drop-in must not pin the v2 format")
	}
}

func TestEnsureMirrorWritesFiles(t *testing.T) {
	root := t.TempDir()
	if err := EnsureMirror(root, "https://m.example"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"etc/k0s/containerd.d/cri-registry.toml", "etc/k0s/containerd.d/certs.d/_default/hosts.toml"} {
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode %o", rel, info.Mode().Perm())
		}
	}
	if err := EnsureMirror(root, "https://m.example"); err != nil {
		t.Fatal("second run must succeed")
	}
}
