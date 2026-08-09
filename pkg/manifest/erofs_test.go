package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestErofsSerialize(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	erofsPipeline := manifest.NewErofs(build, rawImage, "erofs")
	erofsPipeline.SetFilename("image.erofs")

	osbuildPipeline, err := manifest.Serialize(erofsPipeline)
	assert.NoError(t, err)

	assert.Equal(t, "erofs", osbuildPipeline.Name)
	assert.Equal(t, 1, len(osbuildPipeline.Stages))
	erofsStage := osbuildPipeline.Stages[0]
	assert.Equal(t, &osbuild.ErofsStageOptions{
		Filename: "image.erofs",
	}, erofsStage.Options.(*osbuild.ErofsStageOptions))
}

func TestErofsDefaultFilename(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	erofsPipeline := manifest.NewErofs(build, rawImage, "erofs")

	assert.Equal(t, "image.raw", erofsPipeline.Filename())
}

func TestErofsSetFilename(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	erofsPipeline := manifest.NewErofs(build, rawImage, "erofs")

	erofsPipeline.SetFilename("custom.erofs")
	assert.Equal(t, "custom.erofs", erofsPipeline.Filename())
}

func TestErofsGetBuildPackages(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	erofsPipeline := manifest.NewErofs(build, rawImage, "erofs")

	osbuildPipeline, err := manifest.Serialize(erofsPipeline)
	assert.NoError(t, err)
	assert.NotNil(t, osbuildPipeline)
}

func TestErofsExport(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	erofsPipeline := manifest.NewErofs(build, rawImage, "erofs")
	erofsPipeline.SetFilename("exported.erofs")

	art := erofsPipeline.Export()
	assert.NotNil(t, art)
	assert.Equal(t, "exported.erofs", art.Filename())
}
