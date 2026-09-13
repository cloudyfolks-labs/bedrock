package cli

import (
	"bytes"
	"testing"
)

func TestTokenUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"token"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("token create")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestTokenCreateRequiresRoles(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"token", "create"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("--roles")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}
