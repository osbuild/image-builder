package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/runner"
)

func TestZstdSerialize(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	// setup
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	zstdPipeline := manifest.NewZstd(build, rawImage)
	zstdPipeline.SetFilename("filename.zst")

	// run
	osbuildPipeline, err := manifest.Serialize(zstdPipeline)
	assert.NoError(t, err)

	// assert
	assert.Equal(t, "zstd", osbuildPipeline.Name)
	assert.Equal(t, 1, len(osbuildPipeline.Stages))
	zstdStage := osbuildPipeline.Stages[0]
	assert.Equal(t, &osbuild.ZstdStageOptions{
		Filename: "filename.zst",
	}, zstdStage.Options.(*osbuild.ZstdStageOptions))
}
