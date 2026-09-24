package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Resolver func(ctx context.Context, ref string) (string, error)

func RemoteDigest(ctx context.Context, ref string) (string, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return "", err
	}
	desc, err := remote.Get(parsed, remote.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return desc.Digest.String(), nil
}

func ResolveDigests(ctx context.Context, images []string, skipPrefix string, resolve Resolver) (map[string]string, error) {
	pins := map[string]string{}
	for _, image := range images {
		if strings.Contains(image, "@sha256:") || (skipPrefix != "" && strings.HasPrefix(image, skipPrefix)) {
			continue
		}
		digest, err := resolve(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", image, err)
		}
		pins[image] = pinnedReference(image, digest)
	}
	return pins, nil
}

func pinnedReference(image, digest string) string {
	base, _, _ := strings.Cut(image, "@")
	return base + "@" + digest
}

func PinImages(files []rendered, pins map[string]string, skip map[string]struct{}) []rendered {
	toPin := make([]rendered, 0, len(files))
	unchanged := make([]rendered, 0, len(files))
	for _, file := range files {
		if _, found := skip[file.group]; found {
			unchanged = append(unchanged, file)
			continue
		}
		toPin = append(toPin, file)
	}
	pinnedFiles := rewriteImageRefs(toPin, func(current string) (string, bool) {
		pinned, found := pins[current]
		return pinned, found
	})
	return append(pinnedFiles, unchanged...)
}
