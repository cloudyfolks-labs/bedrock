package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestK0sChecksumsDownloadsOncePerArch(t *testing.T) {
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		_, _ = w.Write([]byte("binary for " + r.URL.Path))
	}))
	defer server.Close()
	cache := t.TempDir()
	sums, err := K0sChecksums(context.Background(), server.URL, "v1.36.3+k0s.0", []string{"amd64", "arm64"}, cache)
	if err != nil {
		t.Fatal(err)
	}
	wantAmd := sha256.Sum256([]byte("binary for /v1.36.3+k0s.0/k0s-v1.36.3+k0s.0-amd64"))
	if sums["amd64"] != "sha256:"+hex.EncodeToString(wantAmd[:]) {
		t.Fatalf("amd64 %s", sums["amd64"])
	}
	if _, err := os.Stat(filepath.Join(cache, "k0s", "v1.36.3+k0s.0", "arm64")); err != nil {
		t.Fatal("arm64 binary not cached")
	}
	if _, err := K0sChecksums(context.Background(), server.URL, "v1.36.3+k0s.0", []string{"amd64", "arm64"}, cache); err != nil {
		t.Fatal(err)
	}
	if hits["/v1.36.3+k0s.0/k0s-v1.36.3+k0s.0-amd64"] != 1 || hits["/v1.36.3+k0s.0/k0s-v1.36.3+k0s.0-arm64"] != 1 {
		t.Fatalf("expected one download per arch, got %v", hits)
	}
}

func TestK0sChecksumsCleansPartialDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short body"))
	}))
	defer server.Close()
	cache := t.TempDir()
	if _, err := K0sChecksums(context.Background(), server.URL, "v1.36.3+k0s.0", []string{"amd64"}, cache); err == nil {
		t.Fatal("expected error from partial download")
	}
	partial := filepath.Join(cache, "k0s", "v1.36.3+k0s.0", "amd64.part")
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("expected .part file to be removed, stat err: %v", err)
	}
}

func TestK0sBinaryURL(t *testing.T) {
	got := K0sBinaryURL(DefaultK0sBaseURL, "v1.36.3+k0s.0", "arm64")
	want := "https://github.com/k0sproject/k0s/releases/download/v1.36.3+k0s.0/k0s-v1.36.3+k0s.0-arm64"
	if got != want {
		t.Fatalf("got %s", got)
	}
}
