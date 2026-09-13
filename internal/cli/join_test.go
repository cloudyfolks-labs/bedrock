package cli

import (
	"bytes"
	"testing"
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
