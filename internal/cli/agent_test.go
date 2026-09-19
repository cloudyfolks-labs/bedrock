package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/internal/agent"
	"github.com/cloudyfolks-labs/bedrock/internal/host"
)

func TestRunAgentReloadsClientWhenKubeconfigChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubelet.conf")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	loads := 0
	runs := make(chan struct{}, 4)
	deps := AgentDeps{
		Exec: &host.FakeExec{},
		OSID: "ubuntu",
		Load: func(string) (client.WithWatch, time.Time, error) {
			loads++
			info, err := os.Stat(path)
			if err != nil {
				return nil, time.Time{}, err
			}
			return nil, info.ModTime(), nil
		},
		Run: func(ctx context.Context, _ client.WithWatch, _ agent.Deps) error {
			runs <- struct{}{}
			<-ctx.Done()
			return nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunAgent(ctx, agentOptions{kubeconfig: path, node: "n", root: "/", interval: 20 * time.Millisecond}, deps, &bytes.Buffer{})
	}()
	<-runs
	time.Sleep(30 * time.Millisecond)
	if err := os.WriteFile(path, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runs:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not restart after the kubeconfig changed")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if loads != 2 {
		t.Fatalf("loads %d", loads)
	}
}

func TestRunAgentFailsWhenKubeconfigMissing(t *testing.T) {
	deps := AgentDeps{Exec: &host.FakeExec{}, OSID: "ubuntu", Load: func(string) (client.WithWatch, time.Time, error) { return nil, time.Time{}, errors.New("missing") }}
	if err := RunAgent(context.Background(), agentOptions{kubeconfig: "/nonexistent", node: "n", root: "/", interval: time.Second}, deps, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunAgentReportsUnknownPackageFamily(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubelet.conf")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	family := "unread"
	var log bytes.Buffer
	deps := AgentDeps{
		Exec: &host.FakeExec{},
		OSID: "plan9",
		Load: func(string) (client.WithWatch, time.Time, error) { return nil, time.Time{}, nil },
		Run: func(_ context.Context, _ client.WithWatch, d agent.Deps) error {
			family = d.Packages.Family
			return errors.New("stop")
		},
	}
	if err := RunAgent(context.Background(), agentOptions{kubeconfig: path, node: "n", root: "/", interval: time.Second}, deps, &log); err == nil {
		t.Fatal("expected error")
	}
	if family != "" {
		t.Fatalf("family %q", family)
	}
	if !bytes.Contains(log.Bytes(), []byte("no package manager known")) {
		t.Fatalf("stderr %s", log.String())
	}
}

func TestRunAgentExitsWhenTheLoopReturns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubelet.conf")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := AgentDeps{
		Exec: &host.FakeExec{},
		OSID: "ubuntu",
		Load: func(string) (client.WithWatch, time.Time, error) { return nil, time.Time{}, nil },
		Run:  func(context.Context, client.WithWatch, agent.Deps) error { return nil },
	}
	err := RunAgent(context.Background(), agentOptions{kubeconfig: path, node: "n", root: "/", interval: time.Hour}, deps, &bytes.Buffer{})
	if err == nil || err.Error() != "agent loop exited" {
		t.Fatalf("err %v", err)
	}
}

func TestRunAgentReturnsTheLoopError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubelet.conf")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("watch failed")
	deps := AgentDeps{
		Exec: &host.FakeExec{},
		OSID: "ubuntu",
		Load: func(string) (client.WithWatch, time.Time, error) { return nil, time.Time{}, nil },
		Run:  func(context.Context, client.WithWatch, agent.Deps) error { return want },
	}
	if err := RunAgent(context.Background(), agentOptions{kubeconfig: path, node: "n", root: "/", interval: time.Hour}, deps, &bytes.Buffer{}); !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
}

func TestAgentCommandRejectsNonPositiveInterval(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := agentCommand([]string{"--interval", "0"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("interval")) {
		t.Fatalf("stderr %s", errOut.String())
	}
}
