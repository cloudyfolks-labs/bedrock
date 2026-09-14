package host

import (
	"context"
	"testing"
)

func TestFakeExecResponsePrefixMatch(t *testing.T) {
	e := &FakeExec{ResponsePrefixes: map[string]string{"cosign verify-blob": "ok"}}
	out, err := e.Run(context.Background(), "cosign", "verify-blob", "--bundle", "/tmp/x/SHA256SUMS.sigstore.json")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Fatalf("output %q", out)
	}
}

func TestFakeExecErrorPrefixMatch(t *testing.T) {
	e := &FakeExec{ErrorPrefixes: map[string]error{"cosign verify-blob": &ExitError{Code: 1}}}
	if _, err := e.Run(context.Background(), "cosign", "verify-blob", "--bundle", "/tmp/x/SHA256SUMS.sigstore.json"); err == nil {
		t.Fatal("expected error from prefix match")
	}
}

func TestFakeExecExactMatchWinsOverPrefix(t *testing.T) {
	e := &FakeExec{
		Responses:        map[string]string{"cosign verify-blob exact": "exact"},
		ResponsePrefixes: map[string]string{"cosign verify-blob": "prefix"},
	}
	out, err := e.Run(context.Background(), "cosign", "verify-blob", "exact")
	if err != nil {
		t.Fatal(err)
	}
	if out != "exact" {
		t.Fatalf("output %q, want exact match to win", out)
	}
}

func TestFakeExecNoPrefixMatchFails(t *testing.T) {
	e := &FakeExec{ResponsePrefixes: map[string]string{"cosign verify-blob": "ok"}}
	if _, err := e.Run(context.Background(), "rm", "-rf", "/"); err == nil {
		t.Fatal("unmatched command must fail")
	}
}
