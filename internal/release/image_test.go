package release

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func layerWithFiles(t *testing.T, files map[string]string) v1.Layer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for path, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: path, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(buf.Bytes())), nil })
	if err != nil {
		t.Fatal(err)
	}
	return layer
}

func TestFromImageExtractsReleaseDir(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	ref, err := name.ParseReference(host + "/bedrock:test")
	if err != nil {
		t.Fatal(err)
	}
	layer := layerWithFiles(t, map[string]string{
		"release/release.yaml":               "version: v9\n",
		"release/images.txt":                 "a:1\n",
		"release/manifests/00-crds/crd.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n",
		"release/a/../../x":                  "escape\n",
		"release/dots..ok.yaml":              "ok: true\n",
		"bedrock":                            "binary",
	})
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatal(err)
	}
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{Architecture: "arm64", OS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := FromImage(context.Background(), ref.String(), "arm64", dest); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dest, "release.yaml"))
	if err != nil || string(raw) != "version: v9\n" {
		t.Fatalf("release.yaml %q %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "manifests", "00-crds", "crd.yaml")); err != nil {
		t.Fatal("nested manifest missing")
	}
	if _, err := os.Stat(filepath.Join(dest, "bedrock")); err == nil {
		t.Fatal("files outside release/ must not be extracted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "x")); err == nil {
		t.Fatal("path traversal entries must not escape dest")
	}
	raw, err = os.ReadFile(filepath.Join(dest, "dots..ok.yaml"))
	if err != nil || string(raw) != "ok: true\n" {
		t.Fatalf("dots..ok.yaml %q %v", raw, err)
	}
}
