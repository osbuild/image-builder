package manifest_test

import (
	"math/rand"
	"testing"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestISOTree returns a mock ISOTree pipeline for use in testing
// bootType controls which test bootloaders are created
// ostree controls whether or not the org.osbuild.ostree.grub stage is added
func makeFakeISOTree(bootType manifest.ISOBootType, ostree bool) *manifest.ISOTree {
	m := &manifest.Manifest{}
	runner := &runner.Linux{}
	build := manifest.NewBuild(m, runner, nil, nil)
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	bootloaders := newTestBootloaders(bootType, build, pf, "test-iso", "1")

	rawBootcPipeline := manifest.NewRawBootcImage(build, nil, pf)

	rawBootcPipeline.KernelVersion = "kernel-7.0.8-100.fc43.x86_64"
	rawBootcPipeline.LiveBoot = true

	rootfsPipeline := manifest.NewBootcRootFS(build, rawBootcPipeline, pf)
	isoTree := manifest.NewISOTree(build, rootfsPipeline, bootloaders)
	isoTree.KernelPath = "vmlinuz"
	isoTree.InitramfsPath = "initrd.img"
	isoTree.RootfsPath = "rootfs.img"
	isoTree.SetOSTree = ostree

	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)
	isoTree.PartitionTable = disk.EFIBootPartitionTable(rng)

	return isoTree
}

// copyPaths is used to check org.osbuild.copy
type copyPath struct {
	from string
	to   string
}

func TestISOTreeNoOSTREE(t *testing.T) {
	isoTree := makeFakeISOTree(manifest.Grub2UEFIOnlyISOBoot, false)

	pipeline, err := isoTree.Serialize()
	require.NoError(t, err)

	copyStages := findStages("org.osbuild.copy", pipeline.Stages)
	require.Greater(t, len(copyStages), 0)
	var paths []copyPath
	for _, s := range copyStages {
		copyOptions := s.Options.(*osbuild.CopyStageOptions)
		for _, p := range copyOptions.Paths {
			paths = append(paths, copyPath{from: p.From, to: p.To})
		}
	}
	// Check for the kernel/initrd/rootfs from the ostree deployment
	assert.Contains(t, paths, copyPath{from: "input://tree/vmlinuz", to: "tree:///images/pxeboot/vmlinuz"})
	assert.Contains(t, paths, copyPath{from: "input://tree/initrd.img", to: "tree:///images/pxeboot/initrd.img"})
	assert.Contains(t, paths, copyPath{from: "input://tree/rootfs.img", to: "tree:///LiveOS/squashfs.img"})
	assert.Contains(t, paths, copyPath{from: "input://root-tree/EFI", to: "tree:///"})

	// No ostree.grub2 stage
	require.Nil(t, findStage("org.osbuild.ostree.grub2", pipeline.Stages))
}

func TestISOTreeWithSquashfsOSTREE(t *testing.T) {
	isoTree := makeFakeISOTree(manifest.Grub2UEFIOnlyISOBoot, true)
	isoTree.RootfsType = manifest.SquashfsRootfs

	pipeline, err := isoTree.Serialize()
	require.NoError(t, err)

	// Check the ostree.grub2 stage
	grub2Stage := findStage("org.osbuild.ostree.grub2", pipeline.Stages)
	require.NotNil(t, grub2Stage)
	grub2Opts := grub2Stage.Options.(*osbuild.OSTreeGrub2StageOptions)
	assert.Equal(t, "/EFI/BOOT/grub.cfg", grub2Opts.Filename)
	assert.Equal(t, "mount://-/", grub2Opts.Source)
	// Check ostree.grub2 mount
	assert.Equal(t, "org.osbuild.squashfs", grub2Stage.Mounts[0].Type)
}

func TestISOTreeWithErofsOSTREE(t *testing.T) {
	isoTree := makeFakeISOTree(manifest.Grub2UEFIOnlyISOBoot, true)
	isoTree.RootfsType = manifest.ErofsRootfs

	pipeline, err := isoTree.Serialize()
	require.NoError(t, err)

	// Check the ostree.grub2 stage
	grub2Stage := findStage("org.osbuild.ostree.grub2", pipeline.Stages)
	require.NotNil(t, grub2Stage)
	grub2Opts := grub2Stage.Options.(*osbuild.OSTreeGrub2StageOptions)
	assert.Equal(t, "/EFI/BOOT/grub.cfg", grub2Opts.Filename)
	assert.Equal(t, "mount://-/", grub2Opts.Source)
	// Check ostree.grub2 mount
	assert.Equal(t, "org.osbuild.erofs", grub2Stage.Mounts[0].Type)
}
