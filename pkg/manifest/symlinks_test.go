package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/testdisk"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestOSSymlinks(t *testing.T) {
	p := manifest.NewTestOS()
	p.OSCustomizations.Symlinks = []*fsnode.Symlink{{Path: "/etc/link", Target: "../usr/share/example"}}
	pipeline, err := p.Serialize()
	require.NoError(t, err)
	stage := findStage("org.osbuild.ln", pipeline.Stages)
	require.NotNil(t, stage)
	assert.Equal(t, &osbuild.LnStageOptions{Paths: []osbuild.LnStagePath{
		{LinkName: "tree:///etc/link", Target: "../usr/share/example", Symbolic: true},
	}}, stage.Options)
}

func TestBootcSymlinks(t *testing.T) {
	p := makeFakeRawBootcPipeline()
	p.OSCustomizations.Symlinks = []*fsnode.Symlink{{Path: "/etc/link", Target: "../usr/share/example"}}
	pipeline, err := p.Serialize()
	require.NoError(t, err)
	stage := findStage("org.osbuild.ln", pipeline.Stages)
	require.NotNil(t, stage)
	assertBootcDeploymentAndBindMount(t, stage)
}

func TestOSTreeDeploymentSymlinks(t *testing.T) {
	p := NewTestOSTreeDeployment()
	p.PartitionTable = testdisk.MakeFakePartitionTable("/")
	p.Symlinks = []*fsnode.Symlink{{Path: "/etc/link", Target: "../usr/share/example"}}
	pipeline, err := manifest.SerializeWith(p, testCommitInputs())
	require.NoError(t, err)
	stage := findStage("org.osbuild.ln", pipeline.Stages)
	require.NotNil(t, stage)
	assert.Equal(t, &osbuild.LnStageOptions{Paths: []osbuild.LnStagePath{
		{LinkName: "tree:///etc/link", Target: "../usr/share/example", Symbolic: true},
	}}, stage.Options)
	expected := osbuild.NewLnStage(nil)
	expected.MountOSTree("fedora", "", 0)
	assert.Equal(t, expected.Mounts, stage.Mounts)
}
