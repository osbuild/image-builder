package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/testdisk"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
)

func TestRawImageBootFilesKernelVersion(t *testing.T) {
	const kernelVersion = "7.3.0-65.fc46.aarch64"
	bootFiles := []platform.BootFile{
		{Src: "/usr/lib/modules/{{ .KernelVersion }}/dtb/broadcom/bcm2712-rpi-5-b.dtb", Dst: "/boot/bcm2712-rpi-5-b.dtb"},
		{Src: "/boot/efi/config.txt", Dst: "/boot/config.txt", FromBuild: true},
		{Src: "/firmware/{{ .KernelVersion }}.bin", Dst: "/boot/{{ .KernelVersion }}.bin", FromBuild: true},
	}
	pf := &platform.Data{Arch: arch.ARCH_AARCH64, BootFiles: bootFiles}
	target := NewTestOSWithPlatform(pf)
	target.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot")
	target.PartitionTable.UUID = "D209C89E-EA5E-4FBD-B161-B461CCE297E0"
	target.kernelVer = kernelVersion
	image := NewRawImage(target.BuildPipeline(), target, DiskCustomizations{PartitioningTool: osbuild.PTSgdisk})

	pipeline, err := image.serialize()
	require.NoError(t, err)
	var copies []osbuild.CopyStagePath
	for _, stage := range pipeline.Stages {
		if options, ok := stage.Options.(*osbuild.CopyStageOptions); ok {
			copies = append(copies, options.Paths...)
		}
	}
	assert.Contains(t, copies, osbuild.CopyStagePath{
		From: "input://root-tree/usr/lib/modules/" + kernelVersion + "/dtb/broadcom/bcm2712-rpi-5-b.dtb",
		To:   "mount://-/boot/bcm2712-rpi-5-b.dtb",
	})
	assert.Contains(t, copies, osbuild.CopyStagePath{
		From: "input://build-tree/boot/efi/config.txt", To: "mount://-/boot/config.txt",
	})
	assert.Contains(t, copies, osbuild.CopyStagePath{
		From: "input://build-tree/firmware/" + kernelVersion + ".bin",
		To:   "mount://-/boot/" + kernelVersion + ".bin",
	})
	// Serialization must not bake one kernel version into shared platform data.
	assert.Equal(t, "/usr/lib/modules/{{ .KernelVersion }}/dtb/broadcom/bcm2712-rpi-5-b.dtb", pf.BootFiles[0].Src)
}

func TestBootFilesCopyStageTemplateErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		file     platform.BootFile
		version  string
		expected string
	}{
		{"missing-kernel", platform.BootFile{Src: "/{{ .KernelVersion }}/dtb", Dst: "/boot/dtb"}, "", "cannot render boot file source"},
		{"unknown-value", platform.BootFile{Src: "/{{ .KernelVerison }}/dtb", Dst: "/boot/dtb"}, "7.3.0", "KernelVerison"},
		{"invalid-source", platform.BootFile{Src: "/{{", Dst: "/boot/dtb"}, "7.3.0", "cannot parse boot file path"},
		{"invalid-destination", platform.BootFile{Src: "/file", Dst: "/{{"}, "7.3.0", "cannot render boot file destination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stage, err := bootFilesCopyStage([]platform.BootFile{tc.file}, nil, "root-tree", "disk.img", testdisk.MakeFakePartitionTable("/"), tc.version)
			require.ErrorContains(t, err, tc.expected)
			assert.Nil(t, stage)
		})
	}
}

func TestBootFilesCopyStageLiteralWithoutKernel(t *testing.T) {
	// Existing users, including OSTree images, need no template context for literal paths.
	stage, err := bootFilesCopyStage([]platform.BootFile{{Src: "/file", Dst: "/boot/file"}}, nil, "ostree-tree/commit", "disk.img", testdisk.MakeFakePartitionTable("/"), "")
	require.NoError(t, err)
	assert.Equal(t, []osbuild.CopyStagePath{{From: "input://ostree-tree/commit/file", To: "mount://-/boot/file"}}, stage.Options.(*osbuild.CopyStageOptions).Paths)
}
