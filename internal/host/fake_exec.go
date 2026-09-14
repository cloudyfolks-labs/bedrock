package host

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type FakeExec struct {
	Responses        map[string]string
	Errors           map[string]error
	ResponsePrefixes map[string]string
	ErrorPrefixes    map[string]error
	Calls            []string
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
	if prefix, ok := matchPrefix(key, f.ErrorPrefixes, f.ResponsePrefixes); ok {
		return f.ResponsePrefixes[prefix], f.ErrorPrefixes[prefix]
	}
	return "", fmt.Errorf("no fake response for %q", key)
}

func matchPrefix(key string, errorPrefixes map[string]error, responsePrefixes map[string]string) (string, bool) {
	prefixes := make(map[string]struct{}, len(errorPrefixes)+len(responsePrefixes))
	for prefix := range errorPrefixes {
		prefixes[prefix] = struct{}{}
	}
	for prefix := range responsePrefixes {
		prefixes[prefix] = struct{}{}
	}
	sorted := make([]string, 0, len(prefixes))
	for prefix := range prefixes {
		sorted = append(sorted, prefix)
	}
	sort.Strings(sorted)
	for _, prefix := range sorted {
		if strings.HasPrefix(key, prefix) {
			return prefix, true
		}
	}
	return "", false
}
