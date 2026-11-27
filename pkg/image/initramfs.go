package image

import (
	"fmt"
	"math/rand"
	"slices"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

// Initramfs builds a standalone initramfs.
type Initramfs struct {
	Base
	Customizations manifest.InitramfsTreeCustomizations
	Compression    string
}

func NewInitramfs(platform platform.Platform, filename string) *Initramfs {
	return &Initramfs{Base: NewBase("initramfs", platform, filename)}
}

func (img *Initramfs) InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig,
	runner runner.Runner, _ *rand.Rand) (*artifact.Artifact, error) {
	switch img.Compression {
	case "", "gzip", "zstd":
	default:
		return nil, fmt.Errorf("unsupported initramfs compression %q", img.Compression)
	}

	var initFile *fsnode.File
	for _, file := range slices.Backward(img.Customizations.Files) {
		if file.Path() == "/init" {
			initFile = file
			break
		}
	}
	if initFile != nil {
		mode := initFile.Mode()
		if mode == nil || *mode&0111 == 0 {
			return nil, fmt.Errorf("initramfs /init must be executable")
		}
	} else if len(img.Customizations.PackageSet.Include) == 0 {
		return nil, fmt.Errorf("initramfs requires /init from a package or file customization")
	}

	build := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	build.Checkpoint()
	tree := manifest.NewInitramfsTree(build, img.platform, repos)
	tree.Customizations = img.Customizations

	const output = "initramfs"
	cpioName := output
	if img.Compression != "" {
		cpioName += "-cpio"
	}
	cpio := manifest.NewCPIO(build, tree, cpioName)
	result := GetCompressionPipeline(img.Compression, build, cpio, output)
	if zstd, ok := result.(*manifest.Zstd); ok {
		zstd.Level = 9
	}
	result.SetFilename(img.filename)
	return result.Export(), nil
}
