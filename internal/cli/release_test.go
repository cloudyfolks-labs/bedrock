package cli

import (
	"bytes"
	"testing"
)

func TestReleaseCommandRegistered(t *testing.T) {
	if _, ok := Commands()["release"]; !ok {
		t.Fatal("release command missing")
	}
}

func TestReleaseBuildRequiresFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"release", "build"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("--version")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestReleaseApplyRequiresDir(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"release", "apply"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("--dir")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}
