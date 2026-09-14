package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const DefaultK0sBaseURL = "https://github.com/k0sproject/k0s/releases/download"

func K0sBinaryURL(baseURL, version, arch string) string {
	return fmt.Sprintf("%s/%s/k0s-%s-%s", baseURL, version, version, arch)
}

func K0sChecksums(ctx context.Context, baseURL, version string, arches []string, cacheDir string) (map[string]string, error) {
	sums := map[string]string{}
	for _, arch := range arches {
		path := filepath.Join(cacheDir, "k0s", version, arch)
		if err := ensureDownloaded(ctx, K0sBinaryURL(baseURL, version, arch), path); err != nil {
			return nil, fmt.Errorf("k0s %s %s: %w", version, arch, err)
		}
		sum, err := FileSHA256(path)
		if err != nil {
			return nil, err
		}
		sums[arch] = sum
	}
	return sums, nil
}

func ensureDownloaded(ctx context.Context, url, path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	tmp := path + ".part"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
