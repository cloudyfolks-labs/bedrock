package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	repo := image
	if at := strings.LastIndex(repo, "@"); at >= 0 {
		repo = repo[:at]
	}
	slash := strings.LastIndex(repo, "/")
	if colon := strings.LastIndex(repo, ":"); colon > slash {
		repo = repo[:colon]
	}
	return repo + "@" + digest
}

func PinImages(files []rendered, pins map[string]string) []rendered {
	out := make([]rendered, 0, len(files))
	for _, file := range files {
		objects := make([]*unstructured.Unstructured, 0, len(file.objects))
		for _, obj := range file.objects {
			copy := obj.DeepCopy()
			pinNode(copy.Object, pins)
			objects = append(objects, copy)
		}
		out = append(out, rendered{group: file.group, file: file.file, objects: objects})
	}
	return out
}

func pinNode(node any, pins map[string]string) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "image" {
				if current, ok := child.(string); ok {
					if pinned, found := pins[current]; found {
						value[key] = pinned
					}
					continue
				}
			}
			pinNode(child, pins)
		}
	case []any:
		for _, child := range value {
			pinNode(child, pins)
		}
	}
}
