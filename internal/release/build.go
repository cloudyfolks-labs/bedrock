package release

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/cloudyfolks-labs/bedrock/api/v1alpha1"
)

const bedrockImagePrefix = "ghcr.io/cloudyfolks-labs/bedrock:"

type ComponentConfig struct {
	Name        string   `json:"name"`
	Order       int      `json:"order"`
	Dirs        []string `json:"dirs,omitempty"`
	Chart       string   `json:"chart,omitempty"`
	Version     string   `json:"version,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	Values      string   `json:"values,omitempty"`
	CRDsToGroup string   `json:"crdsToGroup,omitempty"`
	ExtraImages []string `json:"extraImages,omitempty"`
}

type BuildConfig struct {
	K0sVersion  string            `json:"k0sVersion"`
	SupportedOS []string          `json:"supportedOS"`
	UpgradeFrom []string          `json:"upgradeFrom"`
	Components  []ComponentConfig `json:"components"`
}

type BuildOptions struct {
	Version    string
	Image      string
	Out        string
	Helm       string
	Root       string
	K0sBaseURL string
	CacheDir   string
	PinDigests bool
	Resolve    Resolver
}

func LoadBuildConfig(path string) (BuildConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BuildConfig{}, err
	}
	var cfg BuildConfig
	if err := sigyaml.UnmarshalStrict(raw, &cfg); err != nil {
		return BuildConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, component := range cfg.Components {
		if !GroupDirPattern.MatchString(fmt.Sprintf("%02d-%s", component.Order, component.Name)) {
			return BuildConfig{}, fmt.Errorf("component %q has an invalid name or order", component.Name)
		}
	}
	return cfg, nil
}

type rendered struct {
	group   string
	file    string
	objects []*unstructured.Unstructured
}

func groupDir(component ComponentConfig) string {
	return fmt.Sprintf("%02d-%s", component.Order, component.Name)
}

func groupDirs(cfg BuildConfig) map[string]string {
	dirs := map[string]string{}
	for _, component := range cfg.Components {
		dirs[component.Name] = groupDir(component)
	}
	return dirs
}

func validateOut(out string) error {
	if out == "" {
		return fmt.Errorf("out must not be empty")
	}
	if filepath.Clean(out) == "/" {
		return fmt.Errorf("out %q is not a safe output directory", out)
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if abs == cwd {
		return fmt.Errorf("out %q is not a safe output directory", out)
	}
	return nil
}

func Build(cfg BuildConfig, opts BuildOptions) error {
	if err := validateOut(opts.Out); err != nil {
		return err
	}
	dirs := groupDirs(cfg)
	var files []rendered
	for _, component := range cfg.Components {
		out, err := renderComponent(component, dirs, opts)
		if err != nil {
			return fmt.Errorf("component %s: %w", component.Name, err)
		}
		files = append(files, out...)
	}
	files = rewriteBedrockImage(files, opts.Image)
	if opts.PinDigests {
		resolve := opts.Resolve
		if resolve == nil {
			resolve = RemoteDigest
		}
		provisional := loadRenderedGroups(files)
		pins, err := ResolveDigests(context.Background(), mergeImages(ImagesOf(provisional), extraImagesOf(cfg)), opts.Image, resolve)
		if err != nil {
			return err
		}
		files = PinImages(files, pins)
		cfg = pinExtraImages(cfg, pins)
	}
	if err := writeManifests(cfg, files, opts.Out); err != nil {
		return err
	}
	groups, err := loadGroups(os.DirFS(opts.Out))
	if err != nil {
		return err
	}
	spec := v1alpha1.ReleaseSpec{
		Version:     opts.Version,
		Image:       opts.Image,
		K0sVersion:  cfg.K0sVersion,
		UpgradeFrom: cfg.UpgradeFrom,
		SupportedOS: cfg.SupportedOS,
		Components:  componentSpecs(cfg, groups, opts.Version),
	}
	if opts.K0sBaseURL != "" {
		sums, err := K0sChecksums(context.Background(), opts.K0sBaseURL, cfg.K0sVersion, []string{"amd64", "arm64"}, opts.CacheDir)
		if err != nil {
			return err
		}
		spec.K0sChecksums = sums
	}
	return writeMetadata(opts.Out, spec, mergeImages(ImagesOf(groups), extraImagesOf(cfg)))
}

func extraImagesOf(cfg BuildConfig) []string {
	var images []string
	for _, component := range cfg.Components {
		images = append(images, component.ExtraImages...)
	}
	return images
}

func loadRenderedGroups(files []rendered) []Group {
	byGroup := map[string][]*unstructured.Unstructured{}
	for _, file := range files {
		byGroup[file.group] = append(byGroup[file.group], file.objects...)
	}
	groups := make([]Group, 0, len(byGroup))
	for name, objects := range byGroup {
		groups = append(groups, Group{Name: name, Objects: objects})
	}
	return groups
}

func pinExtraImages(cfg BuildConfig, pins map[string]string) BuildConfig {
	components := make([]ComponentConfig, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		extra := make([]string, 0, len(component.ExtraImages))
		for _, image := range component.ExtraImages {
			if pinned, ok := pins[image]; ok {
				image = pinned
			}
			extra = append(extra, image)
		}
		component.ExtraImages = extra
		components = append(components, component)
	}
	cfg.Components = components
	return cfg
}

func mergeImages(images, extra []string) []string {
	seen := map[string]struct{}{}
	merged := make([]string, 0, len(images)+len(extra))
	for _, image := range append(append([]string{}, images...), extra...) {
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		merged = append(merged, image)
	}
	return merged
}

func renderComponent(component ComponentConfig, dirs map[string]string, opts BuildOptions) ([]rendered, error) {
	group := groupDir(component)
	var out []rendered
	for _, dir := range component.Dirs {
		objects, err := loadObjects(os.DirFS(opts.Root), dir, component.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, rendered{group: group, file: component.Name + ".yaml", objects: stripComponentLabel(objects)})
	}
	if component.Chart == "" {
		return out, nil
	}
	objects, err := helmTemplate(component, opts)
	if err != nil {
		return nil, err
	}
	crds, rest := partitionCRDs(objects)
	if component.Namespace != "" && component.Namespace != "kube-system" {
		rest = append([]*unstructured.Unstructured{namespaceObject(component.Namespace)}, rest...)
	}
	if component.CRDsToGroup != "" && len(crds) > 0 {
		target, ok := dirs[component.CRDsToGroup]
		if !ok {
			return nil, fmt.Errorf("crdsToGroup %q is not a component", component.CRDsToGroup)
		}
		out = append(out, rendered{group: target, file: component.Name + "-crds.yaml", objects: crds})
		out = append(out, rendered{group: group, file: component.Name + ".yaml", objects: rest})
		return out, nil
	}
	out = append(out, rendered{group: group, file: component.Name + ".yaml", objects: append(crds, rest...)})
	return out, nil
}

func helmTemplate(component ComponentConfig, opts BuildOptions) ([]*unstructured.Unstructured, error) {
	args := []string{"template", component.Name, filepath.Join(opts.Root, component.Chart), "--include-crds"}
	if component.Namespace != "" {
		args = append(args, "--namespace", component.Namespace)
	}
	if component.Values != "" {
		args = append(args, "-f", filepath.Join(opts.Root, component.Values))
	}
	cmd := exec.Command(opts.Helm, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("helm template: %w: %s", err, stderr.String())
	}
	return splitDocuments(stdout.Bytes())
}

func partitionCRDs(objects []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*unstructured.Unstructured) {
	var crds, rest []*unstructured.Unstructured
	for _, obj := range objects {
		if obj.GetKind() == "CustomResourceDefinition" {
			crds = append(crds, obj)
			continue
		}
		rest = append(rest, obj)
	}
	return crds, rest
}

func namespaceObject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": name}}}
}

func stripComponentLabel(objects []*unstructured.Unstructured) []*unstructured.Unstructured {
	out := make([]*unstructured.Unstructured, 0, len(objects))
	for _, obj := range objects {
		copy := obj.DeepCopy()
		labels := copy.GetLabels()
		delete(labels, ComponentLabel)
		if len(labels) == 0 {
			labels = nil
		}
		copy.SetLabels(labels)
		out = append(out, copy)
	}
	return out
}

func rewriteBedrockImage(files []rendered, image string) []rendered {
	return rewriteImageRefs(files, func(current string) (string, bool) {
		if strings.HasPrefix(current, bedrockImagePrefix) {
			return image, true
		}
		return "", false
	})
}

func rewriteImageRefs(files []rendered, decide func(current string) (string, bool)) []rendered {
	out := make([]rendered, 0, len(files))
	for _, file := range files {
		objects := make([]*unstructured.Unstructured, 0, len(file.objects))
		for _, obj := range file.objects {
			copy := obj.DeepCopy()
			walkImageFields(copy.Object, decide)
			objects = append(objects, copy)
		}
		out = append(out, rendered{group: file.group, file: file.file, objects: objects})
	}
	return out
}

func walkImageFields(node any, decide func(current string) (string, bool)) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "image" {
				if current, ok := child.(string); ok {
					if replacement, found := decide(current); found {
						value[key] = replacement
					}
					continue
				}
			}
			walkImageFields(child, decide)
		}
	case []any:
		for _, child := range value {
			walkImageFields(child, decide)
		}
	}
}

func writeManifests(cfg BuildConfig, files []rendered, out string) error {
	if err := os.RemoveAll(filepath.Join(out, manifestsDir)); err != nil {
		return err
	}
	for _, component := range cfg.Components {
		if err := os.MkdirAll(filepath.Join(out, manifestsDir, fmt.Sprintf("%02d-%s", component.Order, component.Name)), 0o755); err != nil {
			return err
		}
	}
	for _, file := range mergeRendered(files) {
		var buf bytes.Buffer
		for _, obj := range file.objects {
			raw, err := sigyaml.Marshal(obj.Object)
			if err != nil {
				return err
			}
			buf.WriteString("---\n")
			buf.Write(raw)
		}
		if err := os.WriteFile(filepath.Join(out, manifestsDir, file.group, file.file), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

type renderedKey struct {
	group string
	file  string
}

func mergeRendered(files []rendered) []rendered {
	order := make([]renderedKey, 0, len(files))
	objectsByKey := map[renderedKey][]*unstructured.Unstructured{}
	for _, file := range files {
		key := renderedKey{group: file.group, file: file.file}
		if _, seen := objectsByKey[key]; !seen {
			order = append(order, key)
		}
		objectsByKey[key] = append(objectsByKey[key], file.objects...)
	}
	merged := make([]rendered, 0, len(order))
	for _, key := range order {
		merged = append(merged, rendered{group: key.group, file: key.file, objects: objectsByKey[key]})
	}
	return merged
}

func componentSpecs(cfg BuildConfig, groups []Group, version string) []v1alpha1.ReleaseComponent {
	byName := map[string]Group{}
	for _, group := range groups {
		byName[group.Name] = group
	}
	specs := make([]v1alpha1.ReleaseComponent, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		componentVersion := component.Version
		if componentVersion == "" {
			componentVersion = version
		}
		images := ImagesOf([]Group{byName[component.Name]})
		image := ""
		switch {
		case len(images) > 0:
			image = images[0]
		case len(component.ExtraImages) > 0:
			image = component.ExtraImages[0]
		}
		specs = append(specs, v1alpha1.ReleaseComponent{Name: component.Name, Version: componentVersion, Image: image})
	}
	return specs
}

func writeMetadata(out string, spec v1alpha1.ReleaseSpec, images []string) error {
	raw, err := sigyaml.Marshal(spec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, releaseFile), raw, 0o644); err != nil {
		return err
	}
	sorted := append([]string(nil), images...)
	sort.Strings(sorted)
	return os.WriteFile(filepath.Join(out, imagesFile), []byte(strings.Join(sorted, "\n")+"\n"), 0o644)
}
