package manifest_test

import (
	"slices"
	"testing"

	"github.com/osbuild/image-builder/v73/internal/testdisk"
	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/container"
	"github.com/osbuild/image-builder/v73/pkg/manifest"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
	"github.com/osbuild/image-builder/v73/pkg/platform"
	"github.com/osbuild/image-builder/v73/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFakeBootcPXETreePipeline(kernelVersion string, rootfsType manifest.ISORootfsType) *manifest.BootcPXETree {
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
	rootfsPipeline.RootfsType = rootfsType

	err = rootfsPipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	if err != nil {
		panic(err)
	}

	pxetreePipeline := manifest.NewBootcPXETree(build, rootfsPipeline, pf)
	pxetreePipeline.KernelPath = "vmlinuz"
	pxetreePipeline.InitramfsPath = "initrd.img"
	pxetreePipeline.RootfsPath = "rootfs.img"
	pxetreePipeline.RootfsType = rootfsType

	err = pxetreePipeline.SerializeStart(manifest.Inputs{Containers: []container.Spec{{Source: "foo"}}})
	if err != nil {
		panic(err)
	}

	return pxetreePipeline
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
	assert.Contains(t, fromPaths, "input://tree/vmlinuz")
	assert.Contains(t, fromPaths, "input://tree/initrd.img")
	assert.Contains(t, fromPaths, "input://tree/rootfs.img")
	assert.Contains(t, fromPaths, "mount://-/boot/efi/EFI")

	// Check for EFI, grub.cfg and README
	assert.Contains(t, toPaths, "tree:///EFI")
	assert.Contains(t, toPaths, "tree:///grub.cfg")
	assert.Contains(t, toPaths, "tree:///README")

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
	pxeTreePipeline := makeFakeBootcPXETreePipeline(kernelVersion, manifest.SquashfsRootfs)

	pipeline, err := pxeTreePipeline.Serialize()
	require.NoError(t, err)

	// Check the ostree.grub2 stage mount type
	grub2Stage := findStage("org.osbuild.ostree.grub2", pipeline.Stages)
	require.NotNil(t, grub2Stage)
	opts := grub2Stage.Options.(*osbuild.OSTreeGrub2StageOptions)
	assert.Equal(t, "grub.cfg", opts.Filename)
	assert.Equal(t, "mount://-/", opts.Source)
	assert.Equal(t, 0, findMountIdx(grub2Stage.Mounts, "org.osbuild.squashfs"))

	// Check the stages common between squashfs and erofs
	assertCommonPXEStages(t, kernelVersion, pipeline.Stages)
}

func TestBootcPXETreeErofs(t *testing.T) {
	kernelVersion := "5.14.0-611.4.1.el9_7.x86_64"
	pxeTreePipeline := makeFakeBootcPXETreePipeline(kernelVersion, manifest.ErofsRootfs)

	pipeline, err := pxeTreePipeline.Serialize()
	require.NoError(t, err)

	// Check the ostree.grub2 stage mount type
	grub2Stage := findStage("org.osbuild.ostree.grub2", pipeline.Stages)
	require.NotNil(t, grub2Stage)
	opts := grub2Stage.Options.(*osbuild.OSTreeGrub2StageOptions)
	assert.Equal(t, "grub.cfg", opts.Filename)
	assert.Equal(t, "mount://-/", opts.Source)
	assert.Equal(t, 0, findMountIdx(grub2Stage.Mounts, "org.osbuild.erofs"))

	// Check the stages common between squashfs and erofs
	assertCommonPXEStages(t, kernelVersion, pipeline.Stages)
}
