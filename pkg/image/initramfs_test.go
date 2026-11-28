package image_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/image"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild/manifesttest"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestInitramfsRawArtifact(t *testing.T) {
	initPath := filepath.Join(t.TempDir(), "init")
	require.NoError(t, os.WriteFile(initPath, []byte("init"), 0755))
	initFile, err := fsnode.NewFileForURI("/init", nil, nil, nil, initPath)
	require.NoError(t, err)

	img := image.NewInitramfs(&platform.Data{}, "initrd.img")
	img.Customizations.Files = []*fsnode.File{initFile}
	m := manifest.New()
	artifact, err := img.InstantiateManifest(&m, nil, &runner.Fedora{Version: 45}, nil)
	require.NoError(t, err)
	assert.Equal(t, "initramfs", artifact.Export())
	assert.Equal(t, "initrd.img", artifact.Filename())

	chains, err := m.GetPackageSetChains()
	require.NoError(t, err)
	assert.NotContains(t, chains, "initramfs-tree")

	data, err := m.Serialize(nil, nil, nil, nil, nil)
	require.NoError(t, err)
	serialized, err := manifesttest.NewManifestFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, serialized.Pipeline("initramfs"))
	stage := serialized.Pipeline("initramfs").Stage("org.osbuild.cpio.out")
	require.NotNil(t, stage)
	assert.JSONEq(t, `{"filename":"initrd.img","format":"newc"}`, string(stage.Options))
	assert.Contains(t, string(data), "file:"+initPath)

	img.Compression = "zstd"
	m = manifest.New()
	_, err = img.InstantiateManifest(&m, nil, &runner.Fedora{Version: 45}, nil)
	require.NoError(t, err)
	data, err = m.Serialize(nil, nil, nil, nil, nil)
	require.NoError(t, err)
	serialized, err = manifesttest.NewManifestFromBytes(data)
	require.NoError(t, err)
	stage = serialized.Pipeline("initramfs").Stage("org.osbuild.zstd")
	require.NotNil(t, stage)
	assert.JSONEq(t, `{"filename":"initrd.img","level":9}`, string(stage.Options))
}

func TestInitramfsRejectsNonExecutableInit(t *testing.T) {
	img := image.NewInitramfs(&platform.Data{}, "initrd.img")
	mode := os.FileMode(0644)
	initFile, err := fsnode.NewFile("/init", &mode, nil, nil, []byte("init"))
	require.NoError(t, err)
	img.Customizations.Files = []*fsnode.File{initFile}

	m := manifest.New()
	_, err = img.InstantiateManifest(&m, nil, &runner.Fedora{Version: 45}, nil)
	require.ErrorContains(t, err, "/init must be executable")
}
