package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
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

	// Create non-selected compression pipelines first so the selected
	// one ends up last in the manifest pipeline order. This is some form
	// of backwards compatibility with code that uses the last pipeline.
	// Unsure about it; it'll still break for non-compressed exports since
	// they still get all the compression types appended.
	for _, c := range manifest.CompressionTypes {
		if c == compression {
			continue
		}
		manifest.CompressionPipelines[c](buildPipeline, inputPipeline)
	}

	return fn(buildPipeline, inputPipeline)
}
