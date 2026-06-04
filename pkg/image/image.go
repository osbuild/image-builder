package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

type ImageKind interface {
	Name() string
	InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig, runner runner.Runner, rng *rand.Rand) (*artifact.Artifact, error)
}

type Base struct {
	name         string
	platform     platform.Platform
	filename     string
	BuildOptions *manifest.BuildOptions
}

func (img Base) Name() string {
	return img.name
}

func NewBase(name string, platform platform.Platform, filename string) Base {
	return Base{
		name:     name,
		platform: platform,
		filename: filename,
	}
}

func GetCompressionPipeline(compression manifest.Compression, buildPipeline manifest.Build, inputPipeline manifest.FilePipeline) manifest.FilePipeline {
	if compression == "" {
		compression = manifest.CompressionNone
	}
	fn, ok := manifest.CompressionPipelines[compression]
	if !ok {
		panic(fmt.Sprintf("unsupported compression type %q", compression))
	}
	return fn(buildPipeline, inputPipeline)
}
