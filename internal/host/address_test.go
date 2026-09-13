package host

import (
	"context"
	"testing"
)

func TestEnsureAddress(t *testing.T) {
	e := &FakeExec{Responses: map[string]string{"ip addr replace 10.0.10.10/32 dev bond0.10": ""}}
	if err := EnsureAddress(context.Background(), e, "10.0.10.10", "bond0.10"); err != nil {
		t.Fatal(err)
	}
	if len(e.Calls) != 1 || e.Calls[0] != "ip addr replace 10.0.10.10/32 dev bond0.10" {
		t.Fatalf("calls %v", e.Calls)
	}
}

func TestHasAddressTrue(t *testing.T) {
	e := &FakeExec{Responses: map[string]string{"ip -json addr": `[{"addr_info":[{"family":"inet","local":"10.0.10.10"}]}]`}}
	ok, err := HasAddress(context.Background(), e, "10.0.10.10")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected address to be found")
	}
}

func TestHasAddressFalse(t *testing.T) {
	e := &FakeExec{Responses: map[string]string{"ip -json addr": `[{"addr_info":[{"family":"inet","local":"10.0.10.11"}]}]`}}
	ok, err := HasAddress(context.Background(), e, "10.0.10.10")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected address to be absent")
	}
}

func TestFakeExecUnknownCommandFails(t *testing.T) {
	e := &FakeExec{}
	if _, err := e.Run(context.Background(), "rm", "-rf", "/"); err == nil {
		t.Fatal("unknown command must fail")
	}
}
