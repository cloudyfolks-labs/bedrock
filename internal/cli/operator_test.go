package cli

import "testing"

func TestOperatorCommandRegistered(t *testing.T) {
	if _, ok := Commands()["operator"]; !ok {
		t.Fatal("operator command missing")
	}
}
