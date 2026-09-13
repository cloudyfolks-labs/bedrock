package release

import (
	"sort"
)

func ImagesOf(groups []Group) []string {
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, obj := range group.Objects {
			collectImages(obj.Object, seen)
		}
	}
	images := make([]string, 0, len(seen))
	for image := range seen {
		images = append(images, image)
	}
	sort.Strings(images)
	return images
}

func collectImages(node any, seen map[string]struct{}) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "image" {
				if image, ok := child.(string); ok && image != "" {
					seen[image] = struct{}{}
					continue
				}
			}
			collectImages(child, seen)
		}
	case []any:
		for _, child := range value {
			collectImages(child, seen)
		}
	}
}
