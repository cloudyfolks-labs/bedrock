package host

import (
	"context"
	"fmt"
	"strings"
)

type FakeExec struct {
	Responses map[string]string
	Errors    map[string]error
	Calls     []string
}

func (f *FakeExec) Run(_ context.Context, name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.Calls = append(f.Calls, key)
	if err, ok := f.Errors[key]; ok {
		return f.Responses[key], err
	}
	if out, ok := f.Responses[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("no fake response for %q", key)
}
