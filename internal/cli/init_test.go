package cli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
	"github.com/cloudyfolks-labs/bedrock/internal/host"
	"github.com/cloudyfolks-labs/bedrock/internal/k0s"
	"github.com/cloudyfolks-labs/bedrock/internal/roles"
)

const initConfig = `apiVersion: bedrock.cloudyfolks.io/v1alpha1
kind: ClusterConfig
metadata:
  name: lab
spec:
  version: v0.1.0-test
  api:
    vip: 10.0.10.10
  network:
    managementInterface: bond0.10
  storage:
    devices: [/dev/sdb]
  roles: [control-plane, ceph-osd, fabric-gateway, workload]
`

type devInfo struct{ mode fs.FileMode }

func (d devInfo) Name() string       { return "sdb" }
func (d devInfo) Size() int64        { return 0 }
func (d devInfo) Mode() fs.FileMode  { return d.mode }
func (d devInfo) ModTime() time.Time { return time.Time{} }
func (d devInfo) IsDir() bool        { return false }
func (d devInfo) Sys() any           { return nil }

func startEnv(t *testing.T) (client.Client, func(string) (client.Client, error)) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	env := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "manifests", "00-crds")}, ErrorIfCRDPathMissing: true}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Stop() })
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	return c, func(string) (client.Client, error) { return c, nil }
}

func fakeHost(t *testing.T) (string, *host.FakeExec) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"etc", "sys/fs/cgroup", "dev", "run/systemd/system", "var/lib"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(root, "etc", "os-release"), []byte("ID=ubuntu\nVERSION_ID=\"24.04\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "sys", "fs", "cgroup", "cgroup.controllers"), []byte("cpu\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "dev", "kvm"), nil, 0o600)
	e := &host.FakeExec{Responses: map[string]string{
		"ip -json route show default":                 `[{"dst":"default","dev":"bond0.10"}]`,
		"ip -json -4 addr show dev bond0.10":          `[{"addr_info":[{"family":"inet","local":"10.0.10.11"}]}]`,
		"ip -json link show":                          `[{"ifname":"lo"},{"ifname":"bond0.10"}]`,
		"timedatectl show -p NTPSynchronized --value": "yes\n",
		"hostname": "node-1\n",
		"ip addr replace 10.0.10.10/32 dev bond0.10":   "",
		"/usr/local/bin/k0s version":                   "v1.36.3+k0s.0\n",
		"/usr/local/bin/k0s start":                     "",
		"/usr/local/bin/k0s kubectl get --raw=/readyz": "ok",
	}, Errors: map[string]error{"blkid -p -o value -s TYPE /dev/sdb": &host.ExitError{Code: 2}}}
	return root, e
}

func TestRunInitHappyPath(t *testing.T) {
	c, newClient := startEnv(t)
	root, e := fakeHost(t)
	configPath := filepath.Join(root, "cluster.yaml")
	if err := os.WriteFile(configPath, []byte(initConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.IgnoreAlreadyExists(c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}})); err != nil {
		t.Fatal(err)
	}
	_ = c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "bedrock-system"}})
	master := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"fabric/role": "master"}}}
	if err := c.Create(ctx, master); err != nil {
		t.Fatal(err)
	}
	master.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.10.11"}}
	if err := c.Status().Update(ctx, master); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			var cluster v1alpha1.Cluster
			if err := c.Get(ctx, client.ObjectKey{Name: v1alpha1.ClusterName}, &cluster); err == nil && cluster.Status.Version == "" {
				cluster.Status.Version = "v0.1.0-test"
				_ = c.Status().Update(ctx, &cluster)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	dataDir := filepath.Join(root, "var", "lib", "k0s")
	k0sConfigPath := filepath.Join(root, "etc", "k0s", "k0s.yaml")
	installArgs := k0s.InstallArgs(k0s.InstallOptions{
		Role: "controller", ConfigPath: k0sConfigPath, EnableWorker: true, NoTaints: true, DynamicConfig: true,
		Labels:            roles.Labels([]string{"control-plane", "ceph-osd", "fabric-gateway", "workload"}),
		KubeletExtraArgs:  []string{"--node-status-update-frequency=4s"},
		DataDir:           dataDir,
		DisableComponents: k0s.DefaultDisabledComponents,
	})
	e.Responses["/usr/local/bin/k0s "+strings.Join(installArgs, " ")] = ""
	deps := InitDeps{
		Exec:      e,
		Uid:       0,
		FreeBytes: func(string) (uint64, error) { return 100 << 30, nil },
		Stat:      func(string) (fs.FileInfo, error) { return devInfo{fs.ModeDevice}, nil },
		Root:      root,
		NewClient: newClient,
	}
	var out, errOut bytes.Buffer
	code := RunInit(ctx, []string{"-f", configPath, "--release-dir", filepath.Join("..", "release", "testdata", "good"), "--k0s-bin", "/usr/local/bin/k0s", "--data-dir", dataDir, "--timeout", "30s"}, deps, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "etc", "k0s", "k0s.yaml")); err != nil {
		t.Fatal("k0s.yaml not written")
	}
	var cm corev1.ConfigMap
	if err := c.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: "kube-vip"}, &cm); err != nil || cm.Data["address"] != "10.0.10.10" || cm.Data["vip_interface"] != "bond0.10" {
		t.Fatalf("kube-vip configmap %v %v", cm.Data, err)
	}
	var cluster v1alpha1.Cluster
	if err := c.Get(ctx, client.ObjectKey{Name: v1alpha1.ClusterName}, &cluster); err != nil || cluster.Spec.DesiredVersion != "v0.1.0-test" {
		t.Fatalf("cluster %v %v", cluster.Spec, err)
	}
	var h v1alpha1.Host
	if err := c.Get(ctx, client.ObjectKey{Name: "node-1"}, &h); err != nil || len(h.Spec.Roles) != 4 {
		t.Fatalf("host %v %v", h.Spec, err)
	}
	var setting v1alpha1.Setting
	if err := c.Get(ctx, client.ObjectKey{Name: "storage.replicas"}, &setting); err != nil || setting.Spec.Value != "1" {
		t.Fatalf("setting %v %v", setting.Spec, err)
	}
	joined := ""
	for _, call := range e.Calls {
		if len(call) > 30 && call[:30] == "/usr/local/bin/k0s install con" {
			joined = call
		}
	}
	for _, want := range []string{"--enable-worker", "--no-taints", "--enable-dynamic-config", "bedrock.cloudyfolks.io/role-control-plane=true", "fabric/role=master", "--disable-components konnectivity-server,metrics-server,helm"} {
		if !bytes.Contains([]byte(joined), []byte(want)) {
			t.Fatalf("install call %q lacks %q", joined, want)
		}
	}
	if !bytes.Contains(out.Bytes(), []byte("cluster v0.1.0-test ready")) {
		t.Fatalf("stdout %s", out.String())
	}
}

func TestRunInitBlockedByPreflight(t *testing.T) {
	_, newClient := startEnv(t)
	root, e := fakeHost(t)
	configPath := filepath.Join(root, "cluster.yaml")
	if err := os.WriteFile(configPath, []byte(initConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := InitDeps{Exec: e, Uid: 1000, FreeBytes: func(string) (uint64, error) { return 100 << 30, nil }, Stat: func(string) (fs.FileInfo, error) { return devInfo{fs.ModeDevice}, nil }, Root: root, NewClient: newClient}
	var out, errOut bytes.Buffer
	code := RunInit(context.Background(), []string{"-f", configPath, "--release-dir", filepath.Join("..", "release", "testdata", "good"), "--data-dir", filepath.Join(root, "var", "lib", "k0s")}, deps, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("[fail] root")) {
		t.Fatalf("stdout %s", out.String())
	}
	for _, call := range e.Calls {
		if bytes.Contains([]byte(call), []byte("k0s install")) {
			t.Fatal("k0s must not be installed when preflight blocks")
		}
	}
}
