package k0s

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

func TestDownloadVerifiesChecksum(t *testing.T) {
	body := []byte("k0s binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	sum := sha256.Sum256(body)
	dest := filepath.Join(t.TempDir(), "k0s")
	if err := Download(context.Background(), server.URL, dest, "sha256:"+hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("dest %v %v", info, err)
	}
	mismatchDest := filepath.Join(t.TempDir(), "k0s")
	if err := Download(context.Background(), server.URL, mismatchDest, "sha256:00"); err == nil {
		t.Fatal("checksum mismatch must fail")
	}
	if _, err := os.Stat(mismatchDest + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part file must be removed on checksum mismatch, stat err %v", err)
	}
}
