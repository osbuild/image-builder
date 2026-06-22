package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/internal/testdisk"
	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/manifest"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
	"github.com/osbuild/image-builder/v73/pkg/ostree"
	"github.com/osbuild/image-builder/v73/pkg/platform"
	"github.com/osbuild/image-builder/v73/pkg/rpmmd"
	"github.com/osbuild/image-builder/v73/pkg/runner"
)

// Creates a manifest.Inputs with one (empty) commit spec for serializing
// pipelines that require a single ostree commit. The contents of the commit
// spec don't matter.
func testCommitInputs() manifest.Inputs {
	return manifest.Inputs{
		Commits: []ostree.CommitSpec{{}},
	}
}

// NewTestOSTreeDeployment returns a minimally populated OSTreeDeployment for
// use in testing
func NewTestOSTreeDeployment() *manifest.OSTreeDeployment {
	repos := []rpmmd.RepoConfig{}
	m := manifest.New()
	runner := &runner.Fedora{Version: 38}
	build := manifest.NewBuild(&m, runner, repos, nil)
	build.Checkpoint()

	// create an x86_64 platform with bios boot
	platform := &platform.Data{
		Arch:         arch.ARCH_X86_64,
		BIOSPlatform: "i386-pc",
	}
	commit := &ostree.SourceSpec{}
	os := manifest.NewOSTreeCommitDeployment(build, commit, "fedora", platform)
	return os
}

func TestOSTreeDeploymentPipelineFStabStage(t *testing.T) {
	pipeline := NewTestOSTreeDeployment()

	pipeline.PartitionTable = testdisk.MakeFakePartitionTable("/")  // PT specifics don't matter
	pipeline.MountConfiguration = osbuild.MOUNT_CONFIGURATION_FSTAB // set it explicitly just to be sure

	osbuildPipeline, err := manifest.SerializeWith(pipeline, testCommitInputs())
	require.NoError(t, err)
	require.NotNil(t, osbuildPipeline)
	checkStagesForFSTab(t, osbuildPipeline.Stages)
}

func TestOSTreeDeploymentPipelineMountUnitStages(t *testing.T) {
	pipeline := NewTestOSTreeDeployment()

	expectedUnits := []string{"-.mount", "home.mount"}
	pipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/home")
	pipeline.MountConfiguration = osbuild.MOUNT_CONFIGURATION_UNITS

	osbuildPipeline, err := manifest.SerializeWith(pipeline, testCommitInputs())
	require.NoError(t, err)
	require.NotNil(t, osbuildPipeline)
	checkStagesForMountUnits(t, osbuildPipeline.Stages, expectedUnits)
}

func TestOSTreeDeploymentPipelineNoMountUnitStages(t *testing.T) {
	pipeline := NewTestOSTreeDeployment()

	pipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/home")
	pipeline.MountConfiguration = osbuild.MOUNT_CONFIGURATION_NONE

	osbuildPipeline, err := manifest.SerializeWith(pipeline, testCommitInputs())
	require.NoError(t, err)
	require.NotNil(t, osbuildPipeline)
	checkStagesForNoMounts(t, osbuildPipeline.Stages)
}

func TestAddInlineOSTreeDeployment(t *testing.T) {
	deployment := NewTestOSTreeDeployment()

	require := require.New(t)

	// add some files to the Files list which are included near the end of the
	// pipeline
	deployment.Files = createTestFilesForPipeline()

	// enabling FIPS adds files before the Files defined above
	deployment.FIPS = true

	expectedPaths := []string{
		"tree:///etc/system-fips", // from FIPS = true
		"tree:///etc/test/one",    // directly from the OS customizations
		"tree:///etc/test/two",
	}

	// the OSTreeDeployment pipeline *requires* a partition table
	deployment.PartitionTable = testdisk.MakeFakeBtrfsPartitionTable("/")
	pipeline, err := manifest.SerializeWith(deployment, testCommitInputs())
	require.NoError(err)
	require.NotNil(pipeline)

	destinationPaths := collectCopyDestinationPaths(pipeline.Stages)

	// The order is significant. Do not use ElementsMatch() or similar.
	require.Equal(expectedPaths, destinationPaths)

	expectedContents := []string{
		"test 1",
		"test 2",
		"# FIPS module installation complete\n",
	}

	fileContents := manifest.GetInline(deployment)
	// These are used to define the 'sources' part of the manifest, so the
	// order doesn't matter
	require.ElementsMatch(expectedContents, fileContents)
}
