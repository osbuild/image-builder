package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestCPIOPipeline(t *testing.T) {
	tree := manifest.NewTestOS()
	build := tree.BuildPipeline()
	p := manifest.NewCPIO(build, tree, "")

	assert.Equal(t, "cpio", p.Name())
	assert.Equal(t, "image.cpio", p.Filename())
	assert.Empty(t, p.Manifest().GetExports())

	export := p.Export()
	assert.Equal(t, "cpio", export.Export())
	assert.Equal(t, "image.cpio", export.Filename())
	assert.Equal(t, "application/x-cpio", export.MIMEType())
	assert.Equal(t, []string{"cpio"}, p.Manifest().GetExports())

	chains, err := p.Manifest().GetPackageSetChains()
	require.NoError(t, err)
	require.NotEmpty(t, chains[build.Name()])
	assert.Contains(t, chains[build.Name()][0].Include, "cpio")

	pipeline, err := manifest.Serialize(p)
	require.NoError(t, err)
	assert.Equal(t, "cpio", pipeline.Name)
	assert.Equal(t, "name:"+build.Name(), pipeline.Build)
	require.Len(t, pipeline.Stages, 1)
	assert.Equal(t, &osbuild.Stage{
		Type: "org.osbuild.cpio.out",
		Options: &osbuild.CPIOOutStageOptions{
			Filename: "image.cpio",
			Format:   osbuild.CPIOArchiveFormatNewc,
		},
		Inputs: osbuild.NewPipelineTreeInputs("tree", tree.Name()),
	}, pipeline.Stages[0])
}

// Quick test if the CPIO pipeline is a valid input for a compression
// pipeline, maybe overkill?
func TestCPIOCompression(t *testing.T) {
	tree := manifest.NewTestOS()
	build := tree.BuildPipeline()
	cpio := manifest.NewCPIO(build, tree, "archive")
	cpio.SetFilename("payload.cpio")
	p := manifest.NewZstd(build, cpio, "archive-zstd")
	p.SetFilename("payload.cpio.zst")

	pipeline, err := manifest.Serialize(p)
	require.NoError(t, err)
	require.Len(t, pipeline.Stages, 1)
	assert.Equal(t, "org.osbuild.zstd", pipeline.Stages[0].Type)
	assert.Equal(t, &osbuild.ZstdStageOptions{Filename: "payload.cpio.zst"}, pipeline.Stages[0].Options)
	assert.Equal(t, osbuild.NewZstdStageInputs(
		osbuild.NewFilesInputPipelineObjectRef("archive", "payload.cpio", nil),
	), pipeline.Stages[0].Inputs)

	archive, err := manifest.Serialize(cpio)
	require.NoError(t, err)
	require.Len(t, archive.Stages, 1)
	assert.Equal(t, "archive", archive.Name)
	assert.Equal(t, &osbuild.CPIOOutStageOptions{
		Filename: "payload.cpio",
		Format:   osbuild.CPIOArchiveFormatNewc,
	}, archive.Stages[0].Options)
}
