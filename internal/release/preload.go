package release

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func PreloadImages(srcDir, imagesDir string) (int, error) {
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, err
	}
	copied := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar") {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(imagesDir, entry.Name())
		same, err := sameSize(src, dst)
		if err != nil {
			return copied, err
		}
		if same {
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return copied, err
		}
		copied++
	}
	return copied, nil
}

func sameSize(a, b string) (bool, error) {
	infoA, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	infoB, err := os.Stat(b)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return infoA.Size() == infoB.Size(), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	part := dst + ".part"
	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(part)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(part)
		return err
	}
	return os.Rename(part, dst)
}
