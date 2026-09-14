package release

import (
	"archive/tar"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func tarEntries(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := tar.NewReader(file)
	var names []string
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
	}
}

func TestPullLayoutWritesFullIndex(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	ref, err := name.ParseReference(host + "/lib/app:1.0")
	if err != nil {
		t.Fatal(err)
	}
	idx, err := random.Index(128, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.WriteIndex(ref, idx); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir() + "/app.tar"
	digest, err := PullLayout(context.Background(), ref.String(), dest)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := idx.Digest()
	if digest != want.String() {
		t.Fatalf("digest %s want %s", digest, want)
	}
	entries := strings.Join(tarEntries(t, dest), "\n")
	for _, required := range []string{"oci-layout", "index.json", "blobs/sha256/"} {
		if !strings.Contains(entries, required) {
			t.Fatalf("layout tar lacks %s: %s", required, entries)
		}
	}
	manifests, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range manifests.Manifests {
		if !strings.Contains(entries, "blobs/sha256/"+m.Digest.Hex) {
			t.Fatalf("platform manifest %s missing from layout", m.Digest)
		}
	}
}

func TestPullLayoutWrapsSingleImage(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	ref, err := name.ParseReference(host + "/lib/single:1.0")
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir() + "/single.tar"
	digest, err := PullLayout(context.Background(), ref.String(), dest)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := img.Digest()
	if digest != want.String() {
		t.Fatalf("digest %s want %s", digest, want)
	}
	if entries := strings.Join(tarEntries(t, dest), "\n"); !strings.Contains(entries, "index.json") {
		t.Fatal("layout tar lacks index.json")
	}
}

func TestPullLayoutFailsForMissingImage(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	dest := t.TempDir() + "/missing.tar"
	if _, err := PullLayout(context.Background(), host+"/lib/missing:1", dest); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("no partial tar must remain")
	}
}
