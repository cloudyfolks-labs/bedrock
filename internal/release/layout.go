package release

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func PullLayout(ctx context.Context, ref, dest string) (string, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return "", err
	}
	desc, err := remote.Get(parsed, remote.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("pull %s: %w", ref, err)
	}
	index, err := indexOf(desc)
	if err != nil {
		return "", fmt.Errorf("pull %s: %w", ref, err)
	}
	dir, err := os.MkdirTemp(filepath.Dir(dest), ".layout-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if _, err := layout.Write(dir, index); err != nil {
		return "", fmt.Errorf("write layout %s: %w", ref, err)
	}
	if err := tarDirectory(dir, dest); err != nil {
		os.Remove(dest)
		return "", err
	}
	return desc.Digest.String(), nil
}

func indexOf(desc *remote.Descriptor) (v1.ImageIndex, error) {
	if desc.MediaType.IsIndex() {
		return desc.ImageIndex()
	}
	img, err := desc.Image()
	if err != nil {
		return nil, err
	}
	return mutate.AppendManifests(mutate.IndexMediaType(empty.Index, types.OCIImageIndex), mutate.IndexAddendum{Add: img}), nil
}

func tarDirectory(dir, dest string) error {
	part := dest + ".part"
	file, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(file)
	writeErr := writeTarEntries(tw, dir)
	closeErr := firstError(writeErr, tw.Close(), file.Close())
	if closeErr != nil {
		os.Remove(part)
		return closeErr
	}
	return os.Rename(part, dest)
}

func writeTarEntries(tw *tar.Writer, dir string) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if entry.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
}
