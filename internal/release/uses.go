package release

func Uses(bundle Bundle, name string) bool {
	for _, group := range bundle.Groups {
		for _, obj := range group.Objects {
			if usesPlaceholder(obj.Object, name) {
				return true
			}
		}
	}
	return false
}

func usesPlaceholder(node any, name string) bool {
	switch value := node.(type) {
	case string:
		for _, match := range Placeholder.FindAllString(value, -1) {
			if match[2:len(match)-1] == name {
				return true
			}
		}
	case map[string]any:
		for _, child := range value {
			if usesPlaceholder(child, name) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if usesPlaceholder(child, name) {
				return true
			}
		}
	}
	return false
}
