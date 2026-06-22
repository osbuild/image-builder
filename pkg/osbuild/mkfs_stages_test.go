package osbuild

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/internal/testdisk"
	"github.com/osbuild/image-builder/v73/pkg/datasizes"
	"github.com/osbuild/image-builder/v73/pkg/disk"
)

var defaultStageDevices = map[string]Device{
	"device": {
		Type: "org.osbuild.loopback",
		Options: &LoopbackDeviceOptions{
			Filename: "file.img",
			Lock:     true,
		},
	},
}

func TestNewMkfsStage(t *testing.T) {
	devOpts := LoopbackDeviceOptions{
		Filename:   "file.img",
		Start:      0,
		Size:       1024,
		SectorSize: common.ToPtr(uint64(512)),
	}
	device := NewLoopbackDevice(&devOpts)

	devices := map[string]Device{
		"device": *device,
	}

	btrfsOptions := &MkfsBtrfsStageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	mkbtrfs := NewMkfsBtrfsStage(btrfsOptions, devices)
	mkbtrfsExpected := &Stage{
		Type:    "org.osbuild.mkfs.btrfs",
		Options: btrfsOptions,
		Devices: map[string]Device{"device": *device},
	}
	assert.Equal(t, mkbtrfsExpected, mkbtrfs)

	ext4Options := &MkfsExt4StageOptions{
		UUID:   uuid.New().String(),
		Label:  "test",
		Verity: common.ToPtr(true),
	}
	mkext4 := NewMkfsExt4Stage(ext4Options, devices)
	mkext4Expected := &Stage{
		Type:    "org.osbuild.mkfs.ext4",
		Options: ext4Options,
		Devices: map[string]Device{"device": *device},
	}
	assert.Equal(t, mkext4Expected, mkext4)

	fatOptions := &MkfsFATStageOptions{
		VolID:   "7B7795E7",
		Label:   "test",
		FATSize: common.ToPtr(12),
		Geometry: &MkfsFATStageGeometryOptions{
			Heads:           64,
			SectorsPerTrack: 32,
		},
	}
	mkfat := NewMkfsFATStage(fatOptions, devices)
	mkfatExpected := &Stage{
		Type:    "org.osbuild.mkfs.fat",
		Options: fatOptions,
		Devices: map[string]Device{"device": *device},
	}
	assert.Equal(t, mkfatExpected, mkfat)

	xfsOptions := &MkfsXfsStageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	mkxfs := NewMkfsXfsStage(xfsOptions, devices)
	mkxfsExpected := &Stage{
		Type:    "org.osbuild.mkfs.xfs",
		Options: xfsOptions,
		Devices: map[string]Device{"device": *device},
	}
	assert.Equal(t, mkxfsExpected, mkxfs)
}

func TestGenFsStages(t *testing.T) {
	pt := testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi", "swap")
	stages := GenFsStages(pt, "file.img", "build")
	assert.Equal(t, []*Stage{
		{
			Type: "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{
				UUID: disk.RootPartitionUUID,
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{
				UUID: disk.DataPartitionUUID,
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.fat",
			Options: &MkfsFATStageOptions{
				VolID: strings.ReplaceAll(disk.EFIFilesystemUUID, "-", ""),
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkswap",
			Options: &MkswapStageOptions{
				UUID:  "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Label: "swap",
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
	}, stages)
}

func TestGenFsStagesBtrfs(t *testing.T) {
	// Let's put there /extra to make sure that / and /extra creates only one btrfs partition
	pt := testdisk.MakeFakeBtrfsPartitionTable("/", "/boot", "/boot/efi", "/extra", "swap")
	stages := GenFsStages(pt, "file.img", "build")
	assert.Equal(t, []*Stage{
		{
			Type:    "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.fat",
			Options: &MkfsFATStageOptions{
				VolID: strings.ReplaceAll(disk.EFIFilesystemUUID, "-", ""),
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    datasizes.GiB / disk.DefaultSectorSize,
						Size:     100 * datasizes.MiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkswap",
			Options: &MkswapStageOptions{
				UUID:  "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Label: "swap",
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    (datasizes.GiB + 100*datasizes.MiB) / disk.DefaultSectorSize,
						Size:     512 * datasizes.MiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.btrfs",
			Options: &MkfsBtrfsStageOptions{
				UUID: disk.RootPartitionUUID,
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    (512*datasizes.MiB + datasizes.GiB + 100*datasizes.MiB) / disk.DefaultSectorSize,
						Size:     9 * datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.btrfs.subvol",
			Options: &BtrfsSubVolOptions{
				Subvolumes: []BtrfsSubVol{
					{
						Name: "/root",
					},
					{
						Name: "/extra",
					},
				},
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    (512*datasizes.MiB + datasizes.GiB + 100*datasizes.MiB) / disk.DefaultSectorSize,
						Size:     9 * datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
			Mounts: []Mount{
				{
					Name:    "volume",
					Type:    "org.osbuild.btrfs",
					Source:  "device",
					Target:  "/",
					Options: BtrfsMountOptions{},
				},
			},
		},
	}, stages)
}

func TestGenFsStagesLVM(t *testing.T) {
	pt := testdisk.MakeFakeLVMPartitionTable("/", "/boot", "/boot/efi", "/home", "swap")
	stages := GenFsStages(pt, "file.img", "build")
	assert.Equal(t, []*Stage{
		{
			Type:    "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.fat",
			Options: &MkfsFATStageOptions{
				VolID: strings.ReplaceAll(disk.EFIFilesystemUUID, "-", ""),
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    datasizes.GiB / disk.DefaultSectorSize,
						Size:     100 * datasizes.MiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type:    "org.osbuild.mkfs.xfs",
			Options: &MkfsXfsStageOptions{},
			Devices: map[string]Device{
				"rootvg": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    (datasizes.GiB + 100*datasizes.MiB) / disk.DefaultSectorSize,
						Size:     9 * datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
				"device": {
					Type:   "org.osbuild.lvm2.lv",
					Parent: "rootvg",
					Options: &LVM2LVDeviceOptions{
						Volume: "lv-for-/",
					},
				},
			},
		},
		{
			Type:    "org.osbuild.mkfs.xfs",
			Options: &MkfsXfsStageOptions{},
			Devices: map[string]Device{
				"rootvg": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    (datasizes.GiB + 100*datasizes.MiB) / disk.DefaultSectorSize,
						Size:     9 * datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
				"device": {
					Type:   "org.osbuild.lvm2.lv",
					Parent: "rootvg",
					Options: &LVM2LVDeviceOptions{
						Volume: "lv-for-/home",
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkswap",
			Options: &MkswapStageOptions{
				UUID:  "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Label: "swap",
			},
			Devices: map[string]Device{
				"rootvg": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Start:    (datasizes.GiB + 100*datasizes.MiB) / disk.DefaultSectorSize,
						Size:     9 * datasizes.GiB / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
				"device": {
					Type:   "org.osbuild.lvm2.lv",
					Parent: "rootvg",
					Options: &LVM2LVDeviceOptions{
						Volume: "lv-for-swap",
					},
				},
			},
		},
	}, stages)
}

func TestGenFsStagesRaw(t *testing.T) {
	pt := testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi", "raw")
	stages := GenFsStages(pt, "file.img", "build")
	assert.Equal(t, []*Stage{
		{
			Type: "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{
				UUID: disk.RootPartitionUUID,
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{
				UUID: disk.DataPartitionUUID,
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.mkfs.fat",
			Options: &MkfsFATStageOptions{
				VolID: strings.ReplaceAll(disk.EFIFilesystemUUID, "-", ""),
			},
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
		{
			Type: "org.osbuild.write-device",
			Options: &WriteDeviceStageOptions{
				From: "input://tree/usr/lib/modules/5.0/aboot.img",
			},
			Inputs: NewPipelineTreeInputs("tree", "build"),
			Devices: map[string]Device{
				"device": {
					Type: "org.osbuild.loopback",
					Options: &LoopbackDeviceOptions{
						Filename: "file.img",
						Size:     testdisk.FakePartitionSize.Uint64() / disk.DefaultSectorSize,
						Lock:     true,
					},
				},
			},
		},
	}, stages)
}

func TestGenFsStagesUnitExt4Verity(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: "/",
					MkfsOptions: disk.MkfsOptions{
						Verity: true,
					},
				},
			},
		},
	}
	stages := GenFsStages(pt, "file.img", "build")
	assert.Equal(t, []*Stage{
		{
			Type: "org.osbuild.mkfs.ext4",
			Options: &MkfsExt4StageOptions{
				Verity: common.ToPtr(true),
			},
			Devices: defaultStageDevices,
		},
	}, stages)
}

func TestGenFsStagesUnitVfatGeometry(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/boot/efi",
					MkfsOptions: disk.MkfsOptions{
						Geometry: &disk.MkfsOptionGeometry{
							Heads:           64,
							SectorsPerTrack: 32,
						},
					},
				},
			},
		},
	}
	stages := GenFsStages(pt, "file.img", "build")
	assert.Equal(t, []*Stage{
		{
			Type: "org.osbuild.mkfs.fat",
			Options: &MkfsFATStageOptions{
				Geometry: &MkfsFATStageGeometryOptions{
					Heads:           64,
					SectorsPerTrack: 32,
				},
			},
			Devices: defaultStageDevices,
		},
	}, stages)
}

func TestGenFsStagesUnhappy(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type:       "ext2",
					Mountpoint: "/",
				},
			},
		},
	}

	assert.PanicsWithValue(t, "unknown fs type: ext2 for /", func() {
		GenFsStages(pt, "file.img", "build")
	})
}

func TestGenFsStagesUnhappyWrongOptionsVerity(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type: "xfs",
					MkfsOptions: disk.MkfsOptions{
						Verity: true,
					},
				},
			},
		},
	}

	assert.PanicsWithValue(t, "fs type: xfs does not support verity option", func() {
		GenFsStages(pt, "file.img", "build")
	})
}

func TestGenFsStagesUnhappyWrongOptionsGeometry(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Payload: &disk.Filesystem{
					Type: "ext4",
					MkfsOptions: disk.MkfsOptions{
						Geometry: &disk.MkfsOptionGeometry{
							Heads: 16,
						},
					},
				},
			},
		},
	}

	assert.PanicsWithValue(t, "fs type: ext4 does not support geometry option", func() {
		GenFsStages(pt, "file.img", "build")
	})
}
