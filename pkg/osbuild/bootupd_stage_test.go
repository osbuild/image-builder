package osbuild_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

func makeOsbuildMounts(targets ...string) []osbuild.Mount {
	var mnts []osbuild.Mount
	for _, target := range targets {
		fstype := "ext4"
		if strings.HasSuffix(target, "/boot/efi") {
			fstype = "vfat"
		}
		mnts = append(mnts, osbuild.Mount{
			Type:   fmt.Sprintf("org.osbuild.%s", fstype),
			Name:   "mnt-for-" + target,
			Source: "dev-for-" + target,
			Target: target,
		})
	}
	return mnts
}

func makeOsbuildDevices(devnames ...string) map[string]osbuild.Device {
	devices := make(map[string]osbuild.Device)
	for _, devname := range devnames {
		devices[devname] = osbuild.Device{
			Type: "org.osbuild.loopback",
		}
	}
	return devices
}

func TestBootupdStageNewHappy(t *testing.T) {
	opts := &osbuild.BootupdStageOptions{
		StaticConfigs: true,
	}
	devices := makeOsbuildDevices("dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	expectedStage := &osbuild.Stage{
		Type:    "org.osbuild.bootupd",
		Options: opts,
		Devices: devices,
		Mounts:  mounts,
	}
	stage, err := osbuild.NewBootupdStage(opts, devices, mounts, pf)
	require.Nil(t, err)
	assert.Equal(t, stage, expectedStage)
}

func TestBootupdStageMissingMounts(t *testing.T) {
	opts := &osbuild.BootupdStageOptions{
		StaticConfigs: true,
	}
	devices := makeOsbuildDevices("dev-for-/")
	mounts := makeOsbuildMounts("/")
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	stage, err := osbuild.NewBootupdStage(opts, devices, mounts, pf)
	assert.ErrorContains(t, err, "required mounts for bootupd stage [/boot/efi] missing")
	require.Nil(t, stage)
}

func TestBootupdStageMissingDevice(t *testing.T) {
	opts := &osbuild.BootupdStageOptions{
		Bios: &osbuild.BootupdStageOptionsBios{
			Device: "disk",
		},
	}
	devices := makeOsbuildDevices("dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	stage, err := osbuild.NewBootupdStage(opts, devices, mounts, pf)
	assert.ErrorContains(t, err, `cannot find expected device "disk" for bootupd bios option in [dev-for-/ dev-for-/boot dev-for-/boot/efi]`)
	require.Nil(t, stage)
}

func TestBootupdStageJsonHappy(t *testing.T) {
	opts := &osbuild.BootupdStageOptions{
		Deployment: &osbuild.OSTreeDeployment{
			OSName: "default",
			Ref:    "ostree/1/1/0",
		},
		StaticConfigs: true,
		Bios: &osbuild.BootupdStageOptionsBios{
			Device: "disk",
		},
	}
	devices := makeOsbuildDevices("disk", "dev-for-/", "dev-for-/boot", "dev-for-/boot/efi")
	mounts := makeOsbuildMounts("/", "/boot", "/boot/efi")
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	stage, err := osbuild.NewBootupdStage(opts, devices, mounts, pf)
	require.Nil(t, err)
	stageJson, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(stageJson), `{
  "type": "org.osbuild.bootupd",
  "options": {
    "deployment": {
      "osname": "default",
      "ref": "ostree/1/1/0"
    },
    "static-configs": true,
    "bios": {
      "device": "disk"
    }
  },
  "devices": {
    "dev-for-/": {
      "type": "org.osbuild.loopback"
    },
    "dev-for-/boot": {
      "type": "org.osbuild.loopback"
    },
    "dev-for-/boot/efi": {
      "type": "org.osbuild.loopback"
    },
    "disk": {
      "type": "org.osbuild.loopback"
    }
  },
  "mounts": [
    {
      "name": "mnt-for-/",
      "type": "org.osbuild.ext4",
      "source": "dev-for-/",
      "target": "/"
    },
    {
      "name": "mnt-for-/boot",
      "type": "org.osbuild.ext4",
      "source": "dev-for-/boot",
      "target": "/boot"
    },
    {
      "name": "mnt-for-/boot/efi",
      "type": "org.osbuild.vfat",
      "source": "dev-for-/boot/efi",
      "target": "/boot/efi"
    }
  ]
}`)
}

func TestGenBootupdDevicesMountsMissingRoot(t *testing.T) {
	filename := "fake-disk.img"
	pt := &disk.PartitionTable{}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	_, _, err := osbuild.GenBootupdDevicesMounts(filename, pt, pf)
	assert.EqualError(t, err, "required mounts for bootupd stage [/ /boot/efi] missing")
}

func TestGenBootupdDevicesMountsUnexpectedEntity(t *testing.T) {
	filename := "fake-disk.img"
	pt := &disk.PartitionTable{
		Partitions: []disk.Partition{
			{
				Payload: &disk.LUKSContainer{},
			},
		},
	}
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}
	_, _, err := osbuild.GenBootupdDevicesMounts(filename, pt, pf)
	assert.EqualError(t, err, "type *disk.LUKSContainer not supported by bootupd handling yet")
}

var fakePt = &disk.PartitionTable{
	UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
	Type: disk.PT_GPT,
	Partitions: []disk.Partition{
		{
			Size:     1 * datasizes.MebiByte,
			Bootable: true,
			Type:     disk.BIOSBootPartitionGUID,
			UUID:     disk.BIOSBootPartitionUUID,
		},
		{
			Size: 501 * datasizes.MebiByte,
			Type: disk.EFISystemPartitionGUID,
			UUID: disk.EFISystemPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "vfat",
				UUID:         disk.EFIFilesystemUUID,
				Mountpoint:   "/boot/efi",
				Label:        "ESP",
				FSTabOptions: "umask=0077,shortname=winnt",
				FSTabFreq:    0,
				FSTabPassNo:  2,
			},
		},
		{
			Size: 1 * datasizes.GibiByte,
			Type: disk.FilesystemDataGUID,
			UUID: disk.DataPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "ext4",
				Mountpoint:   "/boot",
				Label:        "boot",
				FSTabOptions: "defaults",
				FSTabFreq:    1,
				FSTabPassNo:  2,
			},
		},
		{
			Size: 2 * datasizes.GibiByte,
			Type: disk.FilesystemDataGUID,
			UUID: disk.RootPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "ext4",
				Label:        "root",
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    1,
				FSTabPassNo:  1,
			},
		},
		{
			Size: 2 * datasizes.GibiByte,
			Type: disk.SwapPartitionGUID,
			Payload: &disk.Swap{
				Label: "swap",
			},
		},
	},
}

func TestGenBootupdDevicesMountsHappy(t *testing.T) {
	filename := "fake-disk.img"
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	devices, mounts, err := osbuild.GenBootupdDevicesMounts(filename, fakePt, pf)
	require.Nil(t, err)
	assert.Equal(t, devices, map[string]osbuild.Device{
		"disk": {
			Type: "org.osbuild.loopback",
			Options: &osbuild.LoopbackDeviceOptions{
				Filename: "fake-disk.img",
				Partscan: true,
			},
		},
	})
	assert.Equal(t, mounts, []osbuild.Mount{
		{
			Name:      "-",
			Type:      "org.osbuild.ext4",
			Source:    "disk",
			Target:    "/",
			Partition: common.ToPtr(4),
		},
		{
			Name:      "boot",
			Type:      "org.osbuild.ext4",
			Source:    "disk",
			Target:    "/boot",
			Partition: common.ToPtr(3),
		},
		{
			Name:      "boot-efi",
			Type:      "org.osbuild.fat",
			Source:    "disk",
			Target:    "/boot/efi",
			Partition: common.ToPtr(2),
		},
	})
}

func TestGenBootupdDevicesMountsHappyBtrfs(t *testing.T) {
	filename := "fake-disk.img"
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	devices, mounts, err := osbuild.GenBootupdDevicesMounts(filename, testdisk.MakeFakeBtrfsPartitionTable("/", "/home", "/boot/efi", "/boot"), pf)
	require.Nil(t, err)
	assert.Equal(t, devices, map[string]osbuild.Device{
		"disk": {
			Type: "org.osbuild.loopback",
			Options: &osbuild.LoopbackDeviceOptions{
				Filename: "fake-disk.img",
				Partscan: true,
			},
		},
	})
	assert.Equal(t, []osbuild.Mount{
		{
			Name:      "-",
			Type:      "org.osbuild.btrfs",
			Source:    "disk",
			Target:    "/",
			Options:   osbuild.BtrfsMountOptions{Subvol: "root", Compress: "zstd:1"},
			Partition: common.ToPtr(3),
		},
		{
			Name:      "boot",
			Type:      "org.osbuild.ext4",
			Source:    "disk",
			Target:    "/boot",
			Partition: common.ToPtr(2),
		},
		{
			Name:      "boot-efi",
			Type:      "org.osbuild.fat",
			Source:    "disk",
			Target:    "/boot/efi",
			Partition: common.ToPtr(1),
		},
		{
			Name:      "home",
			Type:      "org.osbuild.btrfs",
			Source:    "disk",
			Target:    "/home",
			Options:   osbuild.BtrfsMountOptions{Subvol: "/home", Compress: "zstd:1"},
			Partition: common.ToPtr(3),
		},
	}, mounts)
}

func TestGenBootupdDevicesMountsHappyLVM(t *testing.T) {
	filename := "fake-disk.img"
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	fakePt := testdisk.MakeFakeLVMPartitionTable("/", "/home", "/boot/efi", "/boot", "swap")

	devices, mounts, err := osbuild.GenBootupdDevicesMounts(filename, fakePt, pf)
	require.Nil(t, err)
	assert.Equal(t, devices, map[string]osbuild.Device{
		"disk": {
			Type: "org.osbuild.loopback",
			Options: &osbuild.LoopbackDeviceOptions{
				Filename: "fake-disk.img",
				Partscan: true,
			},
		},
		"lv-for-/": {
			Type:   "org.osbuild.lvm2.lv",
			Parent: "disk",
			Options: &osbuild.LVM2LVDeviceOptions{
				Volume:    "lv-for-/",
				VGPartnum: common.ToPtr(3),
			},
		},
		"lv-for-/home": {
			Type:   "org.osbuild.lvm2.lv",
			Parent: "disk",
			Options: &osbuild.LVM2LVDeviceOptions{
				Volume:    "lv-for-/home",
				VGPartnum: common.ToPtr(3),
			},
		},
		"lv-for-swap": {
			Type:   "org.osbuild.lvm2.lv",
			Parent: "disk",
			Options: &osbuild.LVM2LVDeviceOptions{
				Volume:    "lv-for-swap",
				VGPartnum: common.ToPtr(3),
			},
		},
	})
	assert.Equal(t, []osbuild.Mount{
		{
			Name:   "-",
			Type:   "org.osbuild.xfs",
			Source: "lv-for-/",
			Target: "/",
		},
		{
			Name:      "boot",
			Type:      "org.osbuild.ext4",
			Source:    "disk",
			Target:    "/boot",
			Partition: common.ToPtr(2),
		},
		{
			Name:      "boot-efi",
			Type:      "org.osbuild.fat",
			Source:    "disk",
			Target:    "/boot/efi",
			Partition: common.ToPtr(1),
		},
		{
			Name:   "home",
			Type:   "org.osbuild.xfs",
			Source: "lv-for-/home",
			Target: "/home",
		},
	}, mounts)
}

func TestGenBootupdDevicesMountsLVM_NotMountableLV(t *testing.T) {
	filename := "fake-disk.img"
	pf := &platform.Data{
		Arch:       arch.ARCH_X86_64,
		UEFIVendor: "test",
	}

	fakePt := testdisk.MakeFakeLVMPartitionTable("/", "/home", "/boot/efi", "/boot")
	fakePt.Partitions[2].Payload.(*disk.LVMVolumeGroup).LogicalVolumes[0].Payload = &disk.LUKSContainer{}

	_, _, err := osbuild.GenBootupdDevicesMounts(filename, fakePt, pf)
	require.Error(t, err)
	require.Regexp(t, `expected LV payload .* to be mountable or swap, got \*disk.LUKSContainer`, err.Error())
}

func TestGenBootupdDevicesMountsSectorSize(t *testing.T) {
	type testCase struct {
		ptSectorSize  uint64
		expectedNil   bool
		expectedValue uint64
	}

	testCases := map[string]testCase{
		"default-sector-size": {
			ptSectorSize: 0,
			expectedNil:  true,
		},
		"sector-size-512": {
			ptSectorSize:  512,
			expectedNil:   false,
			expectedValue: 512,
		},
		"sector-size-4096": {
			ptSectorSize:  4096,
			expectedNil:   false,
			expectedValue: 4096,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			filename := "fake-disk.img"
			pf := &platform.Data{
				Arch:       arch.ARCH_X86_64,
				UEFIVendor: "test",
			}

			fakePt := testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
			fakePt.SectorSize = tc.ptSectorSize

			devices, _, err := osbuild.GenBootupdDevicesMounts(filename, fakePt, pf)
			require.Nil(t, err)

			diskDevice := devices["disk"]
			loopbackOpts := diskDevice.Options.(*osbuild.LoopbackDeviceOptions)

			if tc.expectedNil {
				assert.Nil(t, loopbackOpts.SectorSize)
			} else {
				assert.NotNil(t, loopbackOpts.SectorSize)
				assert.Equal(t, tc.expectedValue, *loopbackOpts.SectorSize)
			}
		})
	}
}
