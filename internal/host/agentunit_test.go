package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentUnit(t *testing.T) {
	unit := AgentUnit("/usr/local/bin/bedrock", "/var/lib/k0s/kubelet.conf")
	for _, want := range []string{"ExecStart=/usr/local/bin/bedrock agent --kubeconfig /var/lib/k0s/kubelet.conf", "Restart=always", "After=network-online.target k0scontroller.service k0sworker.service"} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit lacks %q:\n%s", want, unit)
		}
	}
}

func TestEnsureAgentUnit(t *testing.T) {
	root := t.TempDir()
	exec := &FakeExec{Responses: map[string]string{"systemctl daemon-reload": "", "systemctl enable --now bedrock-agent.service": ""}}
	if err := EnsureAgentUnit(context.Background(), exec, root, "/usr/local/bin/bedrock", "/var/lib/k0s/kubelet.conf"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc", "systemd", "system", "bedrock-agent.service")); err != nil {
		t.Fatal(err)
	}
	if len(exec.Calls) != 2 {
		t.Fatalf("calls %v", exec.Calls)
	}
}

func TestInstallBinary(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "self")
	dest := filepath.Join(dir, "bin", "bedrock")
	if err := os.WriteFile(self, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallBinary(self, dest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("dest %v err %v", info, err)
	}
	if err := os.WriteFile(self, []byte("v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallBinary(self, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "v2" {
		t.Fatalf("dest content %q %v", got, err)
	}
	if err := InstallBinary(dest, dest); err != nil {
		t.Fatal("same path must be a no-op")
	}
}
