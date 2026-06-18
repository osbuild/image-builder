package manifest_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/osbuild/image-builder/internal/testdisk"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFakeBootcRootFSPipeline(kernelVersion string) *manifest.BootcRootFS {
	mani := manifest.New()
	runner := &runner.Linux{}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	build := manifest.NewBuildFromContainer(&mani, runner, nil, nil)
	rawBootcPipeline := manifest.NewRawBootcImage(build, nil, pf)
	rawBootcPipeline.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
	rawBootcPipeline.KernelVersion = kernelVersion
	rawBootcPipeline.LiveBoot = true
	err := rawBootcPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	if err != nil {
		panic(err)
	}

	rootfsPipeline := manifest.NewBootcRootFS(build, rawBootcPipeline, pf)
	err = rootfsPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	if err != nil {
		panic(err)
	}

	return rootfsPipeline
}

// Check for the common stages
func assertCommonRootFSStages(t *testing.T, kernelVersion string, stages []*osbuild.Stage) {
	copyStages := findStages("org.osbuild.copy", stages)
	require.Greater(t, len(copyStages), 0)
	var fromPaths []string
	var toPaths []string
	for _, s := range copyStages {
		copyOptions := s.Options.(*osbuild.CopyStageOptions)
		for _, p := range copyOptions.Paths {
			fromPaths = append(fromPaths, p.From)
			toPaths = append(toPaths, p.To)
		}
	}
	// Check for the kernel/initrd from the ostree deployment
	assert.Contains(t, fromPaths, fmt.Sprintf("mount://-/usr/lib/modules/%s/vmlinuz", kernelVersion))
	assert.Contains(t, toPaths, "tree:///vmlinuz")
	assert.Contains(t, fromPaths, fmt.Sprintf("mount://-/usr/lib/modules/%s/initramfs.img", kernelVersion))
	assert.Contains(t, toPaths, "tree:///initrd.img")

	// Check for the expected chmod paths
	chmodStage := findStage("org.osbuild.chmod", stages)
	require.NotNil(t, chmodStage)
	chmodOpts := chmodStage.Options.(*osbuild.ChmodStageOptions)
	var items []string
	for i := range chmodOpts.Items {
		items = append(items, i)
	}
	slices.Sort(items)
	assert.Equal(t, []string{"/initrd.img", "/rootfs.img", "/vmlinuz"}, items)
}

func TestBootcRootFSSquashfs(t *testing.T) {
	kernelVersion := "5.14.0-611.4.1.el9_7.x86_64"
	rootfsPipeline := makeFakeBootcRootFSPipeline(kernelVersion)

	pipeline, err := rootfsPipeline.Serialize()
	require.NoError(t, err)

	// Check the squashfs stage
	squashfsStage := findStage("org.osbuild.squashfs", pipeline.Stages)
	require.NotNil(t, squashfsStage)
	opts := squashfsStage.Options.(*osbuild.SquashfsStageOptions)
	assert.Equal(t, "rootfs.img", opts.Filename)
	assert.Equal(t, "mount://-/", opts.Source)
	assertNoBootcDeploymentAndBindMount(t, squashfsStage)

	// Check the stages common between squashfs and erofs
	assertCommonRootFSStages(t, kernelVersion, pipeline.Stages)
}

func TestBootcRootFSErofs(t *testing.T) {
	kernelVersion := "5.14.0-611.4.1.el9_7.x86_64"
	pxeTreePipeline := makeFakeBootcRootFSPipeline(kernelVersion)
	pxeTreePipeline.RootfsType = manifest.ErofsRootfs

	pipeline, err := pxeTreePipeline.Serialize()
	require.NoError(t, err)

	// Check the squashfs stage
	erofsStage := findStage("org.osbuild.erofs", pipeline.Stages)
	require.NotNil(t, erofsStage)
	opts := erofsStage.Options.(*osbuild.ErofsStageOptions)
	assert.Equal(t, "rootfs.img", opts.Filename)
	assert.Equal(t, "mount://-/", opts.Source)
	assertNoBootcDeploymentAndBindMount(t, erofsStage)

	// Check the stages common between squashfs and erofs
	assertCommonRootFSStages(t, kernelVersion, pipeline.Stages)
}

// Test to make sure mounts do not include ostree deployment or bind mount
func assertNoBootcDeploymentAndBindMount(t *testing.T, stage *osbuild.Stage) {
	deploymentMntIdx := findMountIdx(stage.Mounts, "org.osbuild.ostree.deployment")
	assert.Equal(t, -1, deploymentMntIdx)
	bindMntIdx := findMountIdx(stage.Mounts, "org.osbuild.bind")
	assert.Equal(t, -1, bindMntIdx)
}
