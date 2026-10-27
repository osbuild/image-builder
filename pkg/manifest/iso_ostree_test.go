package manifest_test

import (
	"math/rand"
	"slices"
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
func makeFakeISOOSTree(bootType manifest.ISOBootType) *manifest.ISOOSTree {
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
	isoOSTree := manifest.NewISOOSTree(build, rootfsPipeline, bootloaders)
	isoOSTree.KernelPath = "vmlinuz"
	isoOSTree.InitramfsPath = "initrd.img"
	isoOSTree.RootfsPath = "rootfs.img"

	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)
	isoOSTree.PartitionTable = disk.EFIBootPartitionTable(rng)

	return isoOSTree
}

func TestISOOSTreeWithSquashfsOSTREE(t *testing.T) {
	isoOSTree := makeFakeISOOSTree(manifest.Grub2UEFIOnlyISOBoot)
	isoOSTree.RootfsType = manifest.SquashfsRootfs

	pipeline, err := isoOSTree.Serialize()
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

func TestISOOSTreeWithErofsOSTREE(t *testing.T) {
	isoOSTree := makeFakeISOOSTree(manifest.Grub2UEFIOnlyISOBoot)
	isoOSTree.RootfsType = manifest.ErofsRootfs

	pipeline, err := isoOSTree.Serialize()
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

func TestISOOSTreeX86(t *testing.T) {
	isoOSTree := makeFakeISOOSTree(manifest.Grub2ISOBoot)
	isoOSTree.RootfsType = manifest.ErofsRootfs

	pipeline, err := isoOSTree.Serialize()
	require.NoError(t, err)

	// There should be two instances of org.osbuild.ostree.grub2
	ostreeStages := findStages("org.osbuild.ostree.grub2", pipeline.Stages)
	require.Equal(t, 2, len(ostreeStages))

	var filenames []string
	for _, s := range ostreeStages {
		o := s.Options.(*osbuild.OSTreeGrub2StageOptions)
		filenames = append(filenames, o.Filename)
	}
	slices.Sort(filenames)
	// The ostree.grub2 stages should be updating the EFI and BIOS paths
	assert.Equal(t, []string{"/EFI/BOOT/grub.cfg", "/boot/grub2/grub.cfg"}, filenames)
}
