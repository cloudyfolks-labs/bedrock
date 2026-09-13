package preflight

import (
	"strings"
	"testing"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
	"github.com/cloudyfolks-labs/bedrock/internal/host"
)

func goodFacts() host.Facts {
	return host.Facts{OSID: "ubuntu", OSVersionID: "24.04", Arch: "amd64", Root: true, Systemd: true, CgroupV2: true, KVM: true, NTPSynced: true, DefaultInterface: "bond0", DefaultIP: "10.0.10.11", Interfaces: []string{"lo", "bond0"}, FreeVarLibBytes: 100 << 30}
}

func goodConfig() v1alpha1.ClusterConfig {
	var cfg v1alpha1.ClusterConfig
	cfg.Spec.API = v1alpha1.APISpec{VIP: "10.0.10.10", VIPMode: "arp"}
	cfg.Spec.Network.ManagementInterface = "bond0"
	cfg.Spec.Storage.Devices = []string{"/dev/sdb"}
	return cfg
}

func TestRunAllGood(t *testing.T) {
	results := Run(goodFacts(), []host.Device{{Path: "/dev/sdb", Block: true}}, goodConfig(), []string{"ubuntu-24.04"})
	if Blocked(results) {
		t.Fatalf("unexpected block: %s", Format(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Fatalf("result not ok: %+v", r)
		}
	}
}

func TestRunFatalFailures(t *testing.T) {
	facts := goodFacts()
	facts.Root = false
	facts.OSID = "gentoo"
	facts.Interfaces = []string{"lo"}
	cfg := goodConfig()
	cfg.Spec.API.VIP = "10.0.10.11"
	results := Run(facts, []host.Device{{Path: "/dev/sdb", Block: true, Signature: "ext4"}}, cfg, []string{"ubuntu-24.04"})
	if !Blocked(results) {
		t.Fatal("expected block")
	}
	failed := map[string]bool{}
	for _, r := range results {
		if !r.OK && r.Fatal {
			failed[r.Name] = true
		}
	}
	for _, name := range []string{"root", "os", "management-interface", "vip", "device-/dev/sdb"} {
		if !failed[name] {
			t.Fatalf("expected %s to fail: %s", name, Format(results))
		}
	}
}

func TestRunWarningsDoNotBlock(t *testing.T) {
	facts := goodFacts()
	facts.KVM = false
	facts.NTPSynced = false
	facts.FreeVarLibBytes = 1 << 30
	results := Run(facts, []host.Device{{Path: "/dev/sdb", Block: true}}, goodConfig(), []string{"ubuntu-24.04"})
	if Blocked(results) {
		t.Fatalf("warnings must not block: %s", Format(results))
	}
	out := Format(results)
	if !strings.Contains(out, "[warn] kvm") || !strings.Contains(out, "[warn] ntp") || !strings.Contains(out, "[warn] disk") {
		t.Fatalf("format %s", out)
	}
}

func TestRunNonBlockDevice(t *testing.T) {
	results := Run(goodFacts(), []host.Device{{Path: "/dev/sdb", Block: false}}, goodConfig(), []string{"ubuntu-24.04"})
	if !Blocked(results) {
		t.Fatal("non-block device must block")
	}
}
