package host

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVIPUnit(t *testing.T) {
	want := "[Unit]\n" +
		"Description=Bedrock API VIP\n" +
		"Before=k0scontroller.service\n" +
		"After=network-online.target\n" +
		"Wants=network-online.target\n" +
		"\n" +
		"[Service]\n" +
		"Type=oneshot\n" +
		"ExecStart=/bin/sh -c 'ping -c 1 -W 1 10.0.10.10 >/dev/null 2>&1 || ip addr replace 10.0.10.10/32 dev bond0.10'\n" +
		"RemainAfterExit=yes\n" +
		"\n" +
		"[Install]\n" +
		"WantedBy=multi-user.target\n"
	if got := VIPUnit("10.0.10.10", "bond0.10"); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestEnsureVIPUnit(t *testing.T) {
	root := t.TempDir()
	e := &FakeExec{Responses: map[string]string{
		"systemctl daemon-reload":              "",
		"systemctl enable bedrock-vip.service": "",
	}}
	if err := EnsureVIPUnit(context.Background(), e, root, "10.0.10.10", "bond0.10"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "etc", "systemd", "system", "bedrock-vip.service")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != VIPUnit("10.0.10.10", "bond0.10") {
		t.Fatalf("unit content %q", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	want := []string{"systemctl daemon-reload", "systemctl enable bedrock-vip.service"}
	if len(e.Calls) != len(want) || e.Calls[0] != want[0] || e.Calls[1] != want[1] {
		t.Fatalf("calls %v", e.Calls)
	}
}
