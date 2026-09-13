package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreloadImagesCopiesTarballs(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "images")
	if err := os.WriteFile(filepath.Join(src, "a.tar"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := PreloadImages(src, dest)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a.tar")); err != nil {
		t.Fatal(err)
	}
	n, err = PreloadImages(src, dest)
	if err != nil || n != 0 {
		t.Fatalf("second run must skip, n=%d err=%v", n, err)
	}
}

func TestPreloadImagesCleansPartialCopy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories do not block root")
	}
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "images")
	if err := os.WriteFile(filepath.Join(src, "a.tar"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dest, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dest, 0o755) })
	if _, err := PreloadImages(src, dest); err == nil {
		t.Fatal("expected error writing into read-only destination")
	}
	if _, err := os.Stat(filepath.Join(dest, "a.tar.part")); !os.IsNotExist(err) {
		t.Fatalf("partial file must not remain, err=%v", err)
	}
}
