package k0s

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudyfolks-labs/bedrock/internal/host"
)

func TestInstallArgsController(t *testing.T) {
	args := InstallArgs(InstallOptions{Role: "controller", ConfigPath: "/etc/k0s/k0s.yaml", EnableWorker: true, NoTaints: true, DynamicConfig: true, Labels: map[string]string{"b": "2", "a": "1"}, KubeletExtraArgs: []string{"--node-status-update-frequency=4s"}, DataDir: "/var/lib/k0s", DisableComponents: DefaultDisabledComponents})
	want := "install controller --config /etc/k0s/k0s.yaml --enable-worker --no-taints --enable-dynamic-config --labels a=1,b=2 --kubelet-extra-args --node-status-update-frequency=4s --data-dir /var/lib/k0s --disable-components konnectivity-server,metrics-server,helm"
	if got := strings.Join(args, " "); got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestInstallArgsWorkerWithToken(t *testing.T) {
	args := InstallArgs(InstallOptions{Role: "worker", TokenFile: "/etc/k0s/join-token", Labels: map[string]string{"x": "y"}, DataDir: "/var/lib/k0s"})
	want := "install worker --token-file /etc/k0s/join-token --labels x=y --data-dir /var/lib/k0s"
	if got := strings.Join(args, " "); got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestClientWaitReady(t *testing.T) {
	e := &host.FakeExec{Responses: map[string]string{"/usr/local/bin/k0s kubectl get --raw=/readyz": "ok"}}
	c := Client{Exec: e, Binary: "/usr/local/bin/k0s", DataDir: "/var/lib/k0s"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestClientWaitReadyTimesOut(t *testing.T) {
	e := &host.FakeExec{Errors: map[string]error{"/usr/local/bin/k0s kubectl get --raw=/readyz": context.DeadlineExceeded}}
	c := Client{Exec: e, Binary: "/usr/local/bin/k0s", DataDir: "/var/lib/k0s", PollInterval: 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := c.WaitReady(ctx); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestClientCreateToken(t *testing.T) {
	e := &host.FakeExec{Responses: map[string]string{"/usr/local/bin/k0s token create --role controller --expiry 1h": "H4sIAAAA\n"}}
	c := Client{Exec: e, Binary: "/usr/local/bin/k0s"}
	tok, err := c.CreateToken(context.Background(), "controller", "1h")
	if err != nil || tok != "H4sIAAAA" {
		t.Fatalf("token %q err %v", tok, err)
	}
}
