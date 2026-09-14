package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestK0sAssetURLs(t *testing.T) {
	if got := K0sAirgapURL("https://x/y", "v1.36.3+k0s.0", "amd64"); got != "https://x/y/v1.36.3+k0s.0/k0s-airgap-bundle-v1.36.3+k0s.0-linux-amd64.tar" {
		t.Fatalf("airgap url %s", got)
	}
	if got := K0sSumsURL("https://x/y", "v1.36.3+k0s.0"); got != "https://x/y/v1.36.3+k0s.0/sha256sums.txt" {
		t.Fatalf("sums url %s", got)
	}
}

func TestParseSums(t *testing.T) {
	text := "abc  k0s-v1-amd64\ndef *k0s-airgap-bundle-v1-linux-amd64.tar\n\nmalformed\n"
	sums := ParseSums(text)
	if sums["k0s-v1-amd64"] != "sha256:abc" || sums["k0s-airgap-bundle-v1-linux-amd64.tar"] != "sha256:def" {
		t.Fatalf("sums %v", sums)
	}
	if len(sums) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(sums))
	}
}

func TestK0sAirgapBundleDownloadsAndVerifies(t *testing.T) {
	content := []byte("airgap-tar")
	sum := sha256.Sum256(content)
	version := "v1.36.3+k0s.0"
	mux := http.NewServeMux()
	mux.HandleFunc("/"+version+"/sha256sums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hex.EncodeToString(sum[:]) + "  k0s-airgap-bundle-" + version + "-linux-amd64.tar\n"))
	})
	mux.HandleFunc("/"+version+"/k0s-airgap-bundle-"+version+"-linux-amd64.tar", func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	cache := t.TempDir()
	path, err := K0sAirgapBundle(context.Background(), server.URL, version, "amd64", cache)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(cache, "k0s")) {
		t.Fatalf("unexpected path %s", path)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(content) {
		t.Fatalf("content %q err %v", got, err)
	}
}

func TestK0sAirgapBundleRejectsBadChecksum(t *testing.T) {
	version := "v1"
	mux := http.NewServeMux()
	mux.HandleFunc("/"+version+"/sha256sums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("0", 64) + "  k0s-airgap-bundle-" + version + "-linux-amd64.tar\n"))
	})
	mux.HandleFunc("/"+version+"/k0s-airgap-bundle-"+version+"-linux-amd64.tar", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("x"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	cache := t.TempDir()
	if _, err := K0sAirgapBundle(context.Background(), server.URL, version, "amd64", cache); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum error, got %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(cache, "k0s", version))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tar") {
			t.Fatal("bad download must not stay in the cache")
		}
	}
}
