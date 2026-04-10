package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/runner"
)

func TestTarSerialize(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	// setup
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	tarPipeline := manifest.NewTar(build, rawImage, "tar-pipeline")
	tarPipeline.SetFilename("filename.tar")
	tarPipeline.Transform = "s/foo/bar"
	tarPipeline.Compression = osbuild.TarArchiveCompressionZstd

	// run
	osbuildPipeline, err := manifest.Serialize(tarPipeline)
	assert.NoError(t, err)

	// assert
	assert.Equal(t, "tar-pipeline", osbuildPipeline.Name)
	assert.Equal(t, 1, len(osbuildPipeline.Stages))
	tarStage := osbuildPipeline.Stages[0]
	assert.Equal(t, &osbuild.TarStageOptions{
		Filename:    "filename.tar",
		Transform:   "s/foo/bar",
		Compression: "zstd",
	}, tarStage.Options.(*osbuild.TarStageOptions))
}
