package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/internal/agent"
	"github.com/cloudyfolks-labs/bedrock/internal/host"
	"github.com/cloudyfolks-labs/bedrock/internal/hostconfig"
	"github.com/cloudyfolks-labs/bedrock/internal/inventory"
	"github.com/cloudyfolks-labs/bedrock/internal/operator"
	"github.com/cloudyfolks-labs/bedrock/internal/pkgmgr"
)

type agentOptions struct {
	kubeconfig string
	node       string
	root       string
	interval   time.Duration
}

type AgentDeps struct {
	Exec host.Exec
	OSID string
	Load func(path string) (client.WithWatch, time.Time, error)
	Run  func(ctx context.Context, c client.WithWatch, deps agent.Deps) error
}

func agentCommand(args []string, _, stderr io.Writer) int {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var o agentOptions
	flags.StringVar(&o.kubeconfig, "kubeconfig", filepath.Join("/var/lib/k0s", "kubelet.conf"), "kubelet kubeconfig with the node identity")
	flags.StringVar(&o.node, "node", "", "node name, default is the lowercase hostname")
	flags.StringVar(&o.root, "root", "/", "host root")
	flags.DurationVar(&o.interval, "interval", 60*time.Second, "reconcile interval")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if o.interval <= 0 {
		fmt.Fprintln(stderr, "agent: --interval must be greater than zero")
		return 2
	}
	if o.node == "" {
		name, err := os.Hostname()
		if err != nil {
			return fail(stderr, err)
		}
		o.node = strings.ToLower(name)
	}
	id, _, err := host.ReadOSRelease(filepath.Join(o.root, "etc", "os-release"))
	if err != nil {
		return fail(stderr, err)
	}
	scheme, err := operator.Scheme()
	if err != nil {
		return fail(stderr, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	deps := AgentDeps{
		Exec: host.RealExec{},
		OSID: id,
		Load: func(path string) (client.WithWatch, time.Time, error) { return agent.LoadClient(path, scheme) },
		Run:  agent.Run,
	}
	if err := RunAgent(ctx, o, deps, stderr); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func RunAgent(ctx context.Context, o agentOptions, deps AgentDeps, stderr io.Writer) error {
	family, err := pkgmgr.Detect(deps.OSID)
	if err != nil {
		fmt.Fprintf(stderr, "agent: %v, package steps will fail\n", err)
	}
	agentDeps := agent.Deps{
		Exec:      deps.Exec,
		Root:      o.root,
		Node:      o.node,
		Now:       time.Now,
		Interval:  o.interval,
		Inventory: inventory.Gather,
		Apply:     hostconfig.Apply,
		Packages:  pkgmgr.Manager{Exec: deps.Exec, Family: family, Root: o.root},
	}
	for {
		c, loaded, err := deps.Load(o.kubeconfig)
		if err != nil {
			return err
		}
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- deps.Run(runCtx, c, agentDeps) }()
		changed := waitForChange(runCtx, o.kubeconfig, loaded, o.interval)
		cancel()
		if err := <-done; err != nil {
			fmt.Fprintf(stderr, "agent: %v\n", err)
		}
		if !changed {
			return nil
		}
	}
}

func waitForChange(ctx context.Context, path string, loaded time.Time, interval time.Duration) bool {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			info, err := os.Stat(path)
			if err == nil && !info.ModTime().Equal(loaded) {
				return true
			}
		}
	}
}
