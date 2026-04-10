package manifest_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFakeBootcPXETreePipeline(kernelVersion string) *manifest.BootcPXETree {
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

	pxetreePipeline := manifest.NewBootcPXETree(build, rawBootcPipeline, pf)
	err = pxetreePipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	if err != nil {
		panic(err)
	}

	return pxetreePipeline
}

// Test to make sure mounts do not include ostree deployment or bind mount
func assertNoBootcDeploymentAndBindMount(t *testing.T, stage *osbuild.Stage) {
	deploymentMntIdx := findMountIdx(stage.Mounts, "org.osbuild.ostree.deployment")
	assert.Equal(t, -1, deploymentMntIdx)
	bindMntIdx := findMountIdx(stage.Mounts, "org.osbuild.bind")
	assert.Equal(t, -1, bindMntIdx)
}

// Check for the common stages
func assertCommonPXEStages(t *testing.T, kernelVersion string, stages []*osbuild.Stage) {
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
	// Check for the kernel/initrd/EFI from the ostree deployment
	assert.Contains(t, fromPaths, fmt.Sprintf("mount://-/usr/lib/modules/%s/vmlinuz", kernelVersion))
	assert.Contains(t, fromPaths, fmt.Sprintf("mount://-/usr/lib/modules/%s/initramfs.img", kernelVersion))
	assert.Contains(t, fromPaths, "mount://-/boot/efi/EFI")

	// Check for EFI, grub.cfg and README
	assert.Contains(t, toPaths, "tree:///EFI")
	assert.Contains(t, toPaths, "tree:///grub.cfg")
	assert.Contains(t, toPaths, "tree:///README")

	// Check the ostree.grub2 stage
	grub2Stage := findStage("org.osbuild.ostree.grub2", stages)
	require.NotNil(t, grub2Stage)
	grub2Opts := grub2Stage.Options.(*osbuild.OSTreeGrub2StageOptions)
	assert.Equal(t, "grub.cfg", grub2Opts.Filename)
	assert.Equal(t, "mount://-/", grub2Opts.Source)

	// Check for the expected chmod paths
	chmodStage := findStage("org.osbuild.chmod", stages)
	require.NotNil(t, chmodStage)
	chmodOpts := chmodStage.Options.(*osbuild.ChmodStageOptions)
	var items []string
	for i := range chmodOpts.Items {
		items = append(items, i)
	}
	slices.Sort(items)
	assert.Equal(t, []string{"/EFI", "/README", "/grub.cfg", "/initrd.img", "/rootfs.img", "/vmlinuz"}, items)

}

func TestBootcPXETreeSquashfs(t *testing.T) {
	kernelVersion := "5.14.0-611.4.1.el9_7.x86_64"
	pxeTreePipeline := makeFakeBootcPXETreePipeline(kernelVersion)

	pipeline, err := pxeTreePipeline.Serialize()
	require.NoError(t, err)

	// Check the squashfs stage
	squashfsStage := findStage("org.osbuild.squashfs", pipeline.Stages)
	require.NotNil(t, squashfsStage)
	opts := squashfsStage.Options.(*osbuild.SquashfsStageOptions)
	assert.Equal(t, "rootfs.img", opts.Filename)
	assert.Equal(t, "mount://-/", opts.Source)
	assertNoBootcDeploymentAndBindMount(t, squashfsStage)

	// Check the stages common between squashfs and erofs
	assertCommonPXEStages(t, kernelVersion, pipeline.Stages)
}

func TestBootcPXETreeErofs(t *testing.T) {
	kernelVersion := "5.14.0-611.4.1.el9_7.x86_64"
	pxeTreePipeline := makeFakeBootcPXETreePipeline(kernelVersion)
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
	assertCommonPXEStages(t, kernelVersion, pipeline.Stages)
}
