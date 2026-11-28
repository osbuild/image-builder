package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestConcatPipeline(t *testing.T) {
	tree := manifest.NewTestOS()
	build := tree.BuildPipeline()
	main := manifest.NewCPIO(build, tree, "main")
	early := manifest.NewCPIO(build, tree, "early")
	p := manifest.NewConcat(build, []manifest.FilePipeline{early, main}, "")
	assert.Equal(t, "concat", p.Name())
	assert.Equal(t, "image.bin", p.Filename())

	p.SetFilename("initrd.img")
	p.Alignment = 4
	pipeline, err := manifest.Serialize(p)
	require.NoError(t, err)
	assert.Equal(t, "concat", pipeline.Name)
	assert.Equal(t, "name:"+build.Name(), pipeline.Build)
	require.Len(t, pipeline.Stages, 1)
	assert.Equal(t, &osbuild.Stage{
		Type: "org.osbuild.concat",
		Options: &osbuild.ConcatStageOptions{
			Filename:  "initrd.img",
			Files:     []string{"input://file.0/image.cpio", "input://file.1/image.cpio"},
			Alignment: 4,
		},
		Inputs: osbuild.ConcatStageInputs{
			"file.0": osbuild.NewFilesInput(osbuild.NewFilesInputPipelineObjectRef("early", "image.cpio", nil)),
			"file.1": osbuild.NewFilesInput(osbuild.NewFilesInputPipelineObjectRef("main", "image.cpio", nil)),
		},
	}, pipeline.Stages[0])
	assert.Empty(t, p.Manifest().GetExports())

	export := p.Export()
	assert.Equal(t, "concat", export.Export())
	assert.Equal(t, "initrd.img", export.Filename())
	assert.Equal(t, []string{"concat"}, p.Manifest().GetExports())
}

func TestConcatPipelineRequiresInputs(t *testing.T) {
	tree := manifest.NewTestOS()
	p := manifest.NewConcat(tree.BuildPipeline(), nil, "")

	_, err := manifest.Serialize(p)
	require.ErrorContains(t, err, "at least one input pipeline")
}
