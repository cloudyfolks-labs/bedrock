package cli

import (
	"bytes"
	"testing"
)

func TestRunUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"nope"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
	if errOut.String() != "unknown command: nope\n" {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestRunNoArgsPrintsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(nil, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("usage: bedrock <command>")) {
		t.Fatalf("stdout %q", out.String())
	}
}

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
	if out.String() != "bedrock dev\n" {
		t.Fatalf("stdout %q", out.String())
	}
}
