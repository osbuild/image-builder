package manifest_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeFakeISOTree returns a mock ISOTree pipeline for use in testing
// bootType controls which test bootloaders are created
func makeFakeISOTree(bootType manifest.ISOBootType, rootfsType manifest.ISORootfsType) *manifest.ISOTree {
	m := &manifest.Manifest{}
	runner := &runner.Linux{}
	build := manifest.NewBuild(m, runner, nil, nil)
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	bootloaders := newTestBootloaders(bootType, build, pf, "test-iso", "1")

	rootfsPipeline := manifest.NewOS(build, pf, []rpmmd.RepoConfig{})
	isoTree := manifest.NewISOTree(build, rootfsPipeline, bootloaders)
	isoTree.KernelPath = "vmlinuz"
	isoTree.InitramfsPath = "initrd.img"
	isoTree.RootfsType = rootfsType

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

func TestISOTreeSquashfs(t *testing.T) {
	for _, tc := range []struct {
		bootType             manifest.ISOBootType
		expectedStageNames   []string
		unexpectedStageNames []string
	}{
		{manifest.Grub2UEFIOnlyISOBoot,
			[]string{"org.osbuild.squashfs"},
			[]string{"org.osbuild.erofs", "org.osbuild.grub2.iso.legacy"}},
		{manifest.Grub2ISOBoot,
			[]string{"org.osbuild.squashfs", "org.osbuild.grub2.iso.legacy"},
			[]string{"org.osbuild.erofs"}},
	} {

		isoTree := makeFakeISOTree(tc.bootType, manifest.SquashfsRootfs)

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
		assert.Contains(t, paths, copyPath{from: "input://root-tree/EFI", to: "tree:///"})

		for _, name := range tc.expectedStageNames {
			assert.NotNil(t, findStage(name, pipeline.Stages), fmt.Sprintf("stage %q not found", name))
		}
		for _, name := range tc.unexpectedStageNames {
			assert.Nil(t, findStage(name, pipeline.Stages), fmt.Sprintf("unexpected stage %q found", name))
		}
	}
}
