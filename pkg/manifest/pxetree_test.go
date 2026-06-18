package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/runner"
)

func newTestPXETree() *manifest.PXETree {
	// Do I use the test OS pipeline for input here?
	os := manifest.NewTestOS()

	m := &manifest.Manifest{}
	runner := &runner.Linux{}
	build := manifest.NewBuild(m, runner, nil, nil)

	return manifest.NewPXETree(build, os)
}

func TestPXETreeSquashfs(t *testing.T) {
	pt := newTestPXETree()
	require.NotNil(t, pt)

	// run
	p, err := manifest.Serialize(pt)
	require.NoError(t, err)
	assert.Greater(t, len(p.Stages), 0)

	copyStages := findStages("org.osbuild.copy", p.Stages)
	require.GreaterOrEqual(t, len(copyStages), 3)

	// First copy stage is the kernel, initrd, EFI tree.
	s := copyStages[0]
	require.NotNil(t, s.Inputs)
	inputs := *s.Inputs.(*osbuild.PipelineTreeInputs)
	_, ok := inputs["tree"]
	require.True(t, ok)
	assert.Equal(t, []string{"name:os"}, inputs["tree"].References)

	// Gather up the paths for this stage
	paths := collectCopyDestinationPaths(copyStages[0:1])
	assert.Equal(t, []string{"tree:///vmlinuz", "tree:///initrd.img", "tree:///EFI"}, paths)

	// Gather up the paths for the 2nd 2 stages
	paths = collectCopyDestinationPaths(copyStages[1:3])
	assert.Equal(t, []string{"tree:///grub.cfg", "tree:///README"}, paths)

	// Does it have a squashfs stage to compress the os tree?
	s = findStage("org.osbuild.squashfs", p.Stages)
	require.NotNil(t, s)
	require.NotNil(t, s.Inputs)
	inputs = *s.Inputs.(*osbuild.PipelineTreeInputs)
	_, ok = inputs["tree"]
	require.True(t, ok)
	assert.Equal(t, []string{"name:os"}, inputs["tree"].References)

	options := *s.Options.(*osbuild.SquashfsStageOptions)
	assert.Equal(t, "rootfs.img", options.Filename)
}

func TestPXETreeErofs(t *testing.T) {
	pt := newTestPXETree()
	require.NotNil(t, pt)
	pt.RootfsType = manifest.ErofsRootfs
	pt.RootfsCompression = "zstd"

	// run
	p, err := manifest.Serialize(pt)
	require.NoError(t, err)
	assert.Greater(t, len(p.Stages), 0)

	copyStages := findStages("org.osbuild.copy", p.Stages)
	require.GreaterOrEqual(t, len(copyStages), 3)

	// First copy stage is the kernel, initrd, EFI tree.
	s := copyStages[0]
	require.NotNil(t, s.Inputs)
	inputs := *s.Inputs.(*osbuild.PipelineTreeInputs)
	_, ok := inputs["tree"]
	require.True(t, ok)
	assert.Equal(t, []string{"name:os"}, inputs["tree"].References)

	// Gather up the paths for this stage
	paths := collectCopyDestinationPaths(copyStages[0:1])
	assert.Equal(t, []string{"tree:///vmlinuz", "tree:///initrd.img", "tree:///EFI"}, paths)

	// Gather up the paths for the 2nd 2 stages
	paths = collectCopyDestinationPaths(copyStages[1:3])
	assert.Equal(t, []string{"tree:///grub.cfg", "tree:///README"}, paths)

	// Does it have a erofs stage to compress the os tree?
	s = findStage("org.osbuild.erofs", p.Stages)
	require.NotNil(t, s)
	require.NotNil(t, s.Inputs)
	inputs = *s.Inputs.(*osbuild.PipelineTreeInputs)
	_, ok = inputs["tree"]
	require.True(t, ok)
	assert.Equal(t, []string{"name:os"}, inputs["tree"].References)

	options := *s.Options.(*osbuild.ErofsStageOptions)
	assert.Equal(t, "rootfs.img", options.Filename)
}
