package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/testregistry"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/stretchr/testify/require"
)

// randOCIArchive generates a random container image and saves it to the given
// path as an OCI archive. Returns the generated image.
func randOCIArchive(t *testing.T, path string) v1.Image {
	t.Helper()

	image, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := image.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cfg.OS = "linux"
	cfg.Architecture = "arm64"
	image, err = mutate.ConfigFile(image, cfg)
	if err != nil {
		t.Fatal(err)
	}

	layoutDir := t.TempDir()
	layoutPath, err := layout.Write(layoutDir, empty.Index)
	if err != nil {
		t.Fatal(err)
	}
	if err := layoutPath.AppendImage(image); err != nil {
		t.Fatal(err)
	}

	outFile, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer outFile.Close()

	tw := tar.NewWriter(outFile)
	defer tw.Close()

	err = filepath.Walk(layoutDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(layoutDir, path)
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path) // nolint:gosec // path symlink traversal is not an issue here
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	return image
}

func TestSimpleUpload(t *testing.T) {
	ref := "test/osbuild:latest"
	require := require.New(t)
	registry := testregistry.New()
	defer registry.Close()

	pushRef := registry.GetRef(ref)

	tmpdir := t.TempDir()
	archivePath := filepath.Join(tmpdir, "container.tar")

	image := randOCIArchive(t, archivePath)
	require.NoError(upload(archivePath, pushRef, "latest", "", "", true))

	// use our own resolver to verify the uploaded image
	resolver := container.NewBlockingResolver("arm64")
	spec := container.SourceSpec{
		Source:    pushRef,
		TLSVerify: common.ToPtr(false),
	}
	res, err := resolver.Resolve(spec)
	require.NoError(err)
	require.NotNil(res)

	digest, err := image.Digest()
	require.NoError(err)
	require.Equal(digest.String(), res.Digest)

	config, err := image.ConfigName()
	require.NoError(err)
	require.Equal(config.String(), res.ImageID)
}
