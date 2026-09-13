package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudyfolks-labs/bedrock/internal/host"
	"github.com/cloudyfolks-labs/bedrock/internal/k0s"
	"github.com/cloudyfolks-labs/bedrock/internal/roles"
)

func TestJoinRequiresToken(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"join"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("--token")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestJoinRejectsGarbageToken(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"join", "--token", "###"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func joinToken(t *testing.T) string {
	t.Helper()
	token, err := k0s.EncodeToken(k0s.Token{
		Version: "v0.1.0-test", Roles: []string{"workload"}, K0sToken: "tok",
		VIP: "10.0.10.10", K0sVersion: "1.36.3+k0s.0", SupportedOS: []string{"ubuntu-24.04"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestRunJoinInstallsWhenNotRunning(t *testing.T) {
	root, e := fakeHost(t)
	dataDir := filepath.Join(root, "var", "lib", "k0s")
	e.Errors["/usr/local/bin/k0s status --data-dir "+dataDir] = &host.ExitError{Code: 1}
	tokenPath := filepath.Join(root, "etc", "k0s", "join-token")
	installArgs := k0s.InstallArgs(k0s.InstallOptions{Role: "worker", Force: true, TokenFile: tokenPath, Labels: roles.Labels([]string{"workload"}), DataDir: dataDir})
	e.Responses["/usr/local/bin/k0s "+strings.Join(installArgs, " ")] = ""
	deps := InitDeps{Exec: e, Uid: 0, FreeBytes: func(string) (uint64, error) { return 100 << 30, nil }, Root: root}
	var out, errOut bytes.Buffer
	code := RunJoin(context.Background(), []string{"--token", joinToken(t), "--data-dir", dataDir, "--k0s-bin", "/usr/local/bin/k0s"}, deps, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, out.String(), errOut.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("joined as workload")) {
		t.Fatalf("stdout %s", out.String())
	}
	installed := false
	for _, call := range e.Calls {
		if call == "/usr/local/bin/k0s "+strings.Join(installArgs, " ") {
			installed = true
		}
	}
	if !installed {
		t.Fatal("expected k0s install to run when not already running")
	}
}

func TestRunJoinSkipsInstallWhenAlreadyRunning(t *testing.T) {
	root, e := fakeHost(t)
	dataDir := filepath.Join(root, "var", "lib", "k0s")
	e.Responses["/usr/local/bin/k0s status --data-dir "+dataDir] = ""
	deps := InitDeps{Exec: e, Uid: 0, FreeBytes: func(string) (uint64, error) { return 100 << 30, nil }, Root: root}
	var out, errOut bytes.Buffer
	code := RunJoin(context.Background(), []string{"--token", joinToken(t), "--data-dir", dataDir, "--k0s-bin", "/usr/local/bin/k0s"}, deps, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, out.String(), errOut.String())
	}
	for _, call := range e.Calls {
		if strings.Contains(call, "k0s install") || call == "/usr/local/bin/k0s start" {
			t.Fatalf("k0s must not be reinstalled or restarted when already running, got %q", call)
		}
	}
}
