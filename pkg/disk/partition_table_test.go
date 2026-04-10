package disk_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/platform"
)

func TestPartitionTable_GetMountpointSize(t *testing.T) {
	pt := testdisk.MakeFakePartitionTable("/", "/app")

	size, err := pt.GetMountpointSize("/")
	assert.NoError(t, err)
	assert.Equal(t, testdisk.FakePartitionSize, size)

	size, err = pt.GetMountpointSize("/app")
	assert.NoError(t, err)
	assert.Equal(t, testdisk.FakePartitionSize, size)

	// non-existing
	_, err = pt.GetMountpointSize("/custom")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot find mountpoint /custom")
}

func TestPartitionTable_GenerateUUIDs(t *testing.T) {
	pt := disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Size:     1 * datasizes.MebiByte,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 2 * datasizes.GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					// create mixed xfs root filesystem and a btrfs /var partition
					Type:         "xfs",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size: 10 * datasizes.GibiByte,
				Payload: &disk.Btrfs{
					Subvolumes: []disk.BtrfsSubvolume{
						{
							Mountpoint: "/var",
						},
					},
				},
			},
		},
	}

	// Static seed for testing
	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))

	pt.GenerateUUIDs(rnd)

	// Check that GenUUID doesn't change already defined UUIDs
	assert.Equal(t, disk.BIOSBootPartitionUUID, pt.Partitions[0].UUID)

	// Check that GenUUID generates fresh UUIDs if not defined prior the call
	assert.Equal(t, "a178892e-e285-4ce1-9114-55780875d64e", pt.Partitions[1].UUID)
	assert.Equal(t, "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", pt.Partitions[1].Payload.(*disk.Filesystem).UUID)

	// Check that GenUUID generates the same UUID for BTRFS and its subvolumes
	assert.Equal(t, "fb180daf-48a7-4ee0-b10d-394651850fd4", pt.Partitions[2].Payload.(*disk.Btrfs).UUID)
	assert.Equal(t, "fb180daf-48a7-4ee0-b10d-394651850fd4", pt.Partitions[2].Payload.(*disk.Btrfs).Subvolumes[0].UUID)
}

func TestPartitionTable_GenerateUUIDs_VFAT(t *testing.T) {
	pt := disk.PartitionTable{
		Type: disk.PT_DOS,
		Partitions: []disk.Partition{
			{
				Size: 2 * datasizes.GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/boot/efi",
				},
			},
		},
	}

	// Static seed for testing
	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))

	pt.GenerateUUIDs(rnd)

	assert.Equal(t, "6E4F-F95F", pt.Partitions[0].Payload.(*disk.Filesystem).UUID)
}

func TestEnsureRootFilesystem(t *testing.T) {
	type testCase struct {
		pt            disk.PartitionTable
		expected      disk.PartitionTable
		defaultFsType disk.FSType
	}

	// use AARCH64 for all test cases
	architecture := arch.ARCH_AARCH64

	testCases := map[string]testCase{
		"empty-plain-gpt": {
			pt:            disk.PartitionTable{Type: disk.PT_GPT},
			defaultFsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     0,
						Type:     disk.RootPartitionAarch64GUID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"empty-plain-dos": {
			pt:            disk.PartitionTable{Type: disk.PT_DOS},
			defaultFsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     0,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"simple-plain-gpt": {
			pt: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			defaultFsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    0,
						Size:     0,
						Type:     disk.RootPartitionAarch64GUID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"simple-plain-dos": {
			pt: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			defaultFsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    0,
						Size:     0,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"simple-lvm": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
										Type:         "ext4",
									},
								},
							},
						},
					},
				},
			},
			defaultFsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "ext4",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
		},
		"simple-btrfs": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
							},
						},
					},
				},
			},
			// no default fs required
			expected: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "root",
									Mountpoint: "/",
								},
							},
						},
					},
				},
			},
		},
		"noop-lvm": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "ext4",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
			expected: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "ext4",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
		},
		"noop-btrfs": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "root",
									Mountpoint: "/",
								},
							},
						},
					},
				},
			},
			expected: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "root",
									Mountpoint: "/",
								},
							},
						},
					},
				},
			},
		},
		"plain-collision": {
			pt: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root",
							Mountpoint:   "/root",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			defaultFsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root",
							Mountpoint:   "/root",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    0,
						Size:     0,
						Type:     disk.RootPartitionAarch64GUID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "root00",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"lvm-collision": {
			pt: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "ext4",
										Mountpoint:   "/root",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
			defaultFsType: disk.FS_XFS,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "ext4",
										Mountpoint:   "/root",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv00",
									Payload: &disk.Filesystem{
										Label:        "root00",
										Type:         "xfs",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
		},
		"btrfs-collision": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "root",
									Mountpoint: "/root",
								},
							},
						},
					},
				},
			},
			expected: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "root",
									Mountpoint: "/root",
								},
								{
									Name:       "root00",
									Mountpoint: "/",
								},
							},
						},
					},
				},
			},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			pt := tc.pt
			err := disk.EnsureRootFilesystem(&pt, tc.defaultFsType, architecture)
			assert.NoError(err)
			assert.Equal(tc.expected, pt)
		})
	}
}

func TestEnsureRootFilesystemErrors(t *testing.T) {
	type testCase struct {
		pt            disk.PartitionTable
		defaultFsType disk.FSType
		errmsg        string
	}

	// use X86_64 for all test cases
	architecture := arch.ARCH_X86_64

	testCases := map[string]testCase{
		"err-empty": {
			pt:     disk.PartitionTable{},
			errmsg: "error creating root partition: no default filesystem type",
		},
		"err-no-pt-type": {
			pt:            disk.PartitionTable{},
			defaultFsType: disk.FS_EXT4,
			errmsg:        "error creating root partition: unknown or unsupported partition table enum: 0",
		},
		"err-plain": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			errmsg: "error creating root partition: no default filesystem type",
		},
		"err-lvm": {
			pt: disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Payload: &disk.LVMVolumeGroup{
							Name: "testvg",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Payload: &disk.Filesystem{
										Label:      "var-log",
										Type:       "xfs",
										Mountpoint: "/var/log",
									},
								},
								{
									Name: "datalv",
									Payload: &disk.Filesystem{
										Label:        "data",
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
										Type:         "ext4",
									},
								},
							},
						},
					},
				},
			},
			errmsg: "error creating root logical volume: no default filesystem type",
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			pt := tc.pt
			err := disk.EnsureRootFilesystem(&pt, tc.defaultFsType, architecture)
			assert.EqualError(err, tc.errmsg)
		})
	}
}

func TestAddBootPartition(t *testing.T) {
	type testCase struct {
		pt       disk.PartitionTable
		expected disk.PartitionTable
		fsType   disk.FSType
		errmsg   string
	}

	testCases := map[string]testCase{
		"empty-plain-gpt": {
			pt:     disk.PartitionTable{Type: disk.PT_GPT},
			fsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"empty-plain-dos": {
			pt:     disk.PartitionTable{Type: disk.PT_DOS},
			fsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     1024 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"simple-plain-gpt": {
			pt: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			fsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    0,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"simple-plain-dos": {
			pt: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			fsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    0,
						Size:     1024 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"label-collision": {
			pt: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/collections/footwear/boot",
							FSTabOptions: "defaults",
						},
					},
				},
			},
			fsType: disk.FS_EXT4,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/collections/footwear/boot",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    0,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						Bootable: false,
						UUID:     "",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot00",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
						},
					},
				},
			},
		},
		"err-nofs": {
			pt:     disk.PartitionTable{},
			errmsg: "error creating boot partition: no filesystem type",
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			pt := tc.pt
			err := disk.AddBootPartition(&pt, tc.fsType)
			if tc.errmsg == "" {
				assert.NoError(err)
				assert.Equal(tc.expected, pt)
			} else {
				assert.EqualError(err, tc.errmsg)
			}
		})
	}
}

func TestAddPartitionsForBootMode(t *testing.T) {
	type testCase struct {
		pt       disk.PartitionTable
		bootMode platform.BootMode
		expected disk.PartitionTable
		errmsg   string
	}

	testCases := map[string]testCase{
		// the partition table type shouldn't matter when the boot mode is
		// none, but let's test with both anyway
		"none-gpt": {
			pt:       disk.PartitionTable{Type: disk.PT_GPT},
			bootMode: platform.BOOT_NONE,
			expected: disk.PartitionTable{Type: disk.PT_GPT},
		},
		"none-dos": {
			pt:       disk.PartitionTable{Type: disk.PT_DOS},
			bootMode: platform.BOOT_NONE,
			expected: disk.PartitionTable{Type: disk.PT_DOS},
		},
		"bios-gpt": {
			pt:       disk.PartitionTable{Type: disk.PT_GPT},
			bootMode: platform.BOOT_LEGACY,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Bootable: true,
						Start:    0,
						Size:     1 * datasizes.MiB,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
				},
			},
		},
		"bios-dos": {
			pt:       disk.PartitionTable{Type: disk.PT_DOS},
			bootMode: platform.BOOT_LEGACY,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Bootable: true,
						Start:    0,
						Size:     1 * datasizes.MiB,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
				},
			},
		},
		"uefi-gpt": {
			pt:       disk.PartitionTable{Type: disk.PT_GPT},
			bootMode: platform.BOOT_UEFI,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Start: 0 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
				},
			},
		},
		"uefi-dos": {
			pt:       disk.PartitionTable{Type: disk.PT_DOS},
			bootMode: platform.BOOT_UEFI,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Start: 0 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
				},
			},
		},
		"hybrid-gpt": {
			pt:       disk.PartitionTable{Type: disk.PT_GPT},
			bootMode: platform.BOOT_HYBRID,
			expected: disk.PartitionTable{
				Type: disk.PT_GPT,
				Partitions: []disk.Partition{
					{
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Size: 200 * datasizes.MiB,
						Type: disk.EFISystemPartitionGUID,
						UUID: disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
				},
			},
		},
		"hybrid-dos": {
			pt:       disk.PartitionTable{Type: disk.PT_DOS},
			bootMode: platform.BOOT_HYBRID,
			expected: disk.PartitionTable{
				Type: disk.PT_DOS,
				Partitions: []disk.Partition{
					{
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Size: 200 * datasizes.MiB,
						Type: disk.EFISystemPartitionDOSID,
						UUID: disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
				},
			},
		},
		"bad-pttype-bios": {
			pt:       disk.PartitionTable{Type: disk.PartitionTableType(911)},
			bootMode: platform.BOOT_LEGACY,
			errmsg:   "error creating BIOS boot partition: unknown or unsupported partition table enum: 911",
		},
		"bad-pttype-uefi": {
			pt:       disk.PartitionTable{Type: disk.PartitionTableType(911)},
			bootMode: platform.BOOT_UEFI,
			errmsg:   "error creating EFI system partition: unknown or unsupported partition table enum: 911",
		},
		"bad-pttype-hybrid": {
			pt:       disk.PartitionTable{Type: disk.PartitionTableType(911)},
			bootMode: platform.BOOT_HYBRID,
			errmsg:   "error creating BIOS boot partition: unknown or unsupported partition table enum: 911",
		},
		"bad-bootmode": {
			pt:       disk.PartitionTable{Type: disk.PT_GPT},
			bootMode: 4,
			errmsg:   "unknown or unsupported boot mode type with enum value 4",
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			pt := tc.pt
			err := disk.AddPartitionsForBootMode(&pt, nil, tc.bootMode, arch.ARCH_X86_64)
			if tc.errmsg == "" {
				assert.NoError(err)
				assert.Equal(tc.expected, pt)
			} else {
				assert.EqualError(err, tc.errmsg)
			}
		})
	}
}

func TestNewCustomPartitionTable(t *testing.T) {
	type testCase struct {
		customizations *blueprint.DiskCustomization
		options        *disk.CustomPartitionTableOptions
		expected       *disk.PartitionTable
	}

	testCases := map[string]testCase{
		"dos-hybrid": {
			customizations: &blueprint.DiskCustomization{
				Type: "dos", // overrides the default option
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_AARCH64, // doesn't matter for dos
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: 201 * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start: 1 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    201 * datasizes.MiB,
						Size:     0,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
						},
					},
				},
			},
		},
		"plain-dos": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:  20 * datasizes.MiB,
						PartType: "42", // overrides the inferred type
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: 222 * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     "42",
						Bootable: false,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    222 * datasizes.MiB,
						Size:     0,
						Type:     disk.FilesystemLinuxDOSID,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain-gpt": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:   20 * datasizes.MiB,
						PartType:  "01234567-89ab-cdef-0123-456789abcdef", // overrides the inferred type
						PartLabel: "TheLabel",
						PartUUID:  "76543210-89ab-cdef-0123-456789abcdef", // don't generate uuid
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: 223 * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     "01234567-89ab-cdef-0123-456789abcdef",
						Bootable: false,
						UUID:     "76543210-89ab-cdef-0123-456789abcdef",
						Label:    "TheLabel",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    222 * datasizes.MiB,
						Size:     1*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)),
						Type:     disk.RootPartitionX86_64GUID,
						UUID:     "a178892e-e285-4ce1-9114-55780875d64e",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain-none": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
					{
						MinSize: 5 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							FSType: "none",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_AARCH64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: (1+200+20+5)*datasizes.MiB + datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start: 1 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    201 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						UUID:     "a178892e-e285-4ce1-9114-55780875d64e",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    221 * datasizes.MiB,
						Size:     5 * datasizes.MiB,
						Type:     disk.FilesystemDataGUID,
						UUID:     "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Bootable: false,
						Payload:  nil,
					},
					{
						Start:    226 * datasizes.MiB,
						Size:     1*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.RootPartitionAarch64GUID,
						UUID:     "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain+swap": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
					{
						MinSize: 5 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Label:  "swap",
							FSType: "swap",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_AARCH64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: 226*datasizes.MiB + datasizes.MiB, // last part + footer
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start: 1 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    201 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						UUID:     "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    221 * datasizes.MiB,
						Size:     5 * datasizes.MiB,
						Type:     disk.SwapPartitionGUID,
						UUID:     "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
						Bootable: false,
						Payload: &disk.Swap{
							Label:        "swap",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    226 * datasizes.MiB,
						Size:     1*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.RootPartitionAarch64GUID,
						UUID:     "32f3a8ae-b79e-4856-b659-c18f0dcecc77",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "a178892e-e285-4ce1-9114-55780875d64e",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain-legacy": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_LEGACY,
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: 22 * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start:    2 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    22 * datasizes.MiB,
						Size:     0,
						Type:     disk.FilesystemLinuxDOSID,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain-uefi": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_UEFI,
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: 221 * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start: 1 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    201 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    221 * datasizes.MiB,
						Size:     0,
						Type:     disk.FilesystemLinuxDOSID,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain-reqsizes": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
				StartOffset: 3 * datasizes.MiB,
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				RequiredMinSizes:   map[string]datasizes.Size{"/": 1 * datasizes.GiB, "/usr": 2 * datasizes.GiB}, // the default for our distro definitions
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type:        disk.PT_DOS,
				Size:        225*datasizes.MiB + 3*datasizes.GiB,
				StartOffset: 3 * datasizes.MiB,
				UUID:        "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    4 * datasizes.MiB, // header + offset
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 5 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    205 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    225 * datasizes.MiB,
						Size:     3 * datasizes.GiB,
						Type:     disk.FilesystemLinuxDOSID,
						UUID:     "", // partitions on dos PTs don't have UUIDs
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain+s390x": {
			customizations: &blueprint.DiskCustomization{
				Type: "gpt", // overrides the default option
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 50 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							Label:      "root",
							FSType:     "xfs",
						},
					},
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/home",
							Label:      "home",
							FSType:     "ext4",
						},
					},
					{
						MinSize: 12 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Label:  "swappyswaps",
							FSType: "swap",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_EXT4,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_DOS,
				RequiredMinSizes:   map[string]datasizes.Size{"/": 3 * datasizes.GiB},
				Architecture:       arch.ARCH_S390X,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: (20+12+1)*datasizes.MiB + 3*datasizes.GiB + datasizes.MiB, // start + size of last partition + footer

				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB,
						Size:     20 * datasizes.MiB,
						Type:     disk.FilesystemDataGUID,
						UUID:     "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start: (20 + 1) * datasizes.MiB,
						Size:  12 * datasizes.MiB,
						Type:  disk.SwapPartitionGUID,
						UUID:  "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
						Payload: &disk.Swap{
							Label:        "swappyswaps",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    (20 + 12 + 1) * datasizes.MiB,
						Size:     3*datasizes.GiB + datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.RootPartitionS390xGUID,
						UUID:     "32f3a8ae-b79e-4856-b659-c18f0dcecc77",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
							UUID:         "a178892e-e285-4ce1-9114-55780875d64e",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain+ppc64le+dos": {
			customizations: &blueprint.DiskCustomization{
				Type: "dos", // overrides the default option
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 50 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							Label:      "root",
							FSType:     "xfs",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_EXT4,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_DOS,
				RequiredMinSizes:   map[string]datasizes.Size{"/": 3 * datasizes.GiB},
				Architecture:       arch.ARCH_PPC64LE,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: 4*datasizes.MiB + 3*datasizes.GiB + datasizes.MiB, // start + size of last partition + footer

				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     4 * datasizes.MiB,
						Bootable: true,
						Type:     disk.PRepPartitionDOSID,
					},
					// root is aligned to the end but not reindexed
					{
						Start:    5 * datasizes.MiB,
						Size:     3 * datasizes.GiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain+ppc64le+gpt": {
			customizations: &blueprint.DiskCustomization{
				Type: "gpt", // overrides the default option
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 50 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							Label:      "root",
							FSType:     "xfs",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_EXT4,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_DOS,
				RequiredMinSizes:   map[string]datasizes.Size{"/": 3 * datasizes.GiB},
				Architecture:       arch.ARCH_PPC64LE,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: (4+1)*datasizes.MiB + 3*datasizes.GiB + datasizes.MiB, // start + size of last partition + footer

				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     4 * datasizes.MiB,
						Bootable: true,
						Type:     disk.PRePartitionGUID,
						UUID:     "fb180daf-48a7-4ee0-b10d-394651850fd4",
					},
					// root is aligned to the end but not reindexed
					{
						Start:    5 * datasizes.MiB,
						Size:     3*datasizes.GiB + datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.RootPartitionPpc64leGUID,
						UUID:     "a178892e-e285-4ce1-9114-55780875d64e",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"lvm-dos": {
			customizations: &blueprint.DiskCustomization{
				Type: "dos",
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "lvm",
						MinSize: 100 * datasizes.MiB,
						VGCustomization: blueprint.VGCustomization{
							Name: "testvg",
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name:    "varloglv",
									MinSize: 10 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/var/log",
										Label:      "var-log",
										FSType:     "xfs",
									},
								},
								{
									Name:    "rootlv",
									MinSize: 50 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/",
										Label:      "root",
										FSType:     "xfs",
									},
								},
								{ // unnamed + untyped logical volume
									MinSize: 100 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/data",
										Label:      "data",
										FSType:     "ext4", // TODO: remove when we reintroduce the default fs
									},
								},
								{ // swap on LV
									Name:    "swaplv",
									MinSize: 30 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Label:  "swap-on-lv",
										FSType: "swap",
									},
								},
							},
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType: disk.FS_EXT4,
				BootMode:      platform.BOOT_HYBRID,
				Architecture:  arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Size: 1226*datasizes.MiB + 200*datasizes.MiB, // start + size of last partition (VG)
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     1024 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						UUID:     "",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    1226 * datasizes.MiB,
						Size:     200 * datasizes.MiB, // the sum of the LVs (rounded to the next 4 MiB extent)
						Type:     disk.LVMPartitionDOSID,
						UUID:     "",
						Bootable: false,
						Payload: &disk.LVMVolumeGroup{
							Name:        "testvg",
							Description: "created via lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Size: 12 * datasizes.MiB, // rounded up to next extent (4 MiB)
									Payload: &disk.Filesystem{
										Label:        "var-log",
										Type:         "xfs",
										Mountpoint:   "/var/log",
										FSTabOptions: "defaults",
										UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
									},
								},
								{
									Name: "rootlv",
									Size: 52 * datasizes.MiB, // rounded up to the next extent (4 MiB)
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "xfs",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
										UUID:         "a178892e-e285-4ce1-9114-55780875d64e",
									},
								},
								{
									Name: "datalv",
									Size: 100 * datasizes.MiB,
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4", // the defaultType
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
										UUID:         "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
									},
								},
								{
									Name: "swaplv",
									Size: 32 * datasizes.MiB, // rounded up to the next extent (4 MiB)
									Payload: &disk.Swap{
										Label:        "swap-on-lv",
										UUID:         "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
		},
		"lvm-gpt": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "lvm",
						MinSize: 100 * datasizes.MiB,
						VGCustomization: blueprint.VGCustomization{
							Name: "testvg",
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name:    "varloglv",
									MinSize: 10 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/var/log",
										Label:      "var-log",
										FSType:     "xfs",
									},
								},
								{
									Name:    "rootlv",
									MinSize: 50 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/",
										Label:      "root",
										FSType:     "xfs",
									},
								},
								{ // unnamed + untyped logical volume
									MinSize: 100 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/data",
										Label:      "data",
										FSType:     "ext4", // TODO: remove when we reintroduce the default fs
									},
								},
								{ // swap on LV
									Name:    "swaplv",
									MinSize: 30 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Label:  "swap-on-lv",
										FSType: "swap",
									},
								},
							},
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType: disk.FS_EXT4,
				BootMode:      platform.BOOT_HYBRID,
				Architecture:  arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT, // default when unspecified
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Size: 1226*datasizes.MiB + 200*datasizes.MiB + datasizes.MiB, // start + size of last partition (VG) + footer
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						UUID:     "32f3a8ae-b79e-4856-b659-c18f0dcecc77",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    1226 * datasizes.MiB,
						Size:     200*datasizes.MiB + datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // the sum of the LVs (rounded to the next 4 MiB extent) grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.LVMPartitionGUID,
						UUID:     "c75e7a81-bfde-475f-a7cf-e242cf3cc354",
						Bootable: false,
						Payload: &disk.LVMVolumeGroup{
							Name:        "testvg",
							Description: "created via lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Size: 12 * datasizes.MiB, // rounded up to next extent (4 MiB)
									Payload: &disk.Filesystem{
										Label:        "var-log",
										Type:         "xfs",
										Mountpoint:   "/var/log",
										FSTabOptions: "defaults",
										UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
									},
								},
								{
									Name: "rootlv",
									Size: 52 * datasizes.MiB, // rounded up to the next extent (4 MiB)
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "xfs",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
										UUID:         "a178892e-e285-4ce1-9114-55780875d64e",
									},
								},
								{
									Name: "datalv",
									Size: 100 * datasizes.MiB,
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4", // the defaultType
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
										UUID:         "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
									},
								},
								{
									Name: "swaplv",
									Size: 32 * datasizes.MiB, // rounded up to the next extent (4 MiB)
									Payload: &disk.Swap{
										Label:        "swap-on-lv",
										UUID:         "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
		},
		"lvm-multivg": {
			// two volume groups, both unnamed, and no root lv defined
			// NOTE: this is currently not supported by customizations but the
			// PT creation function can handle it
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "lvm",
						MinSize: 100 * datasizes.MiB,
						VGCustomization: blueprint.VGCustomization{
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name:    "varloglv",
									MinSize: 10 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/var/log",
										Label:      "var-log",
										FSType:     "xfs",
									},
								},
							},
						},
					},
					{
						Type: "lvm",
						VGCustomization: blueprint.VGCustomization{
							LogicalVolumes: []blueprint.LVCustomization{
								{ // unnamed + untyped logical volume
									MinSize: 100 * datasizes.MiB,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/data",
										Label:      "data",
										FSType:     "ext4", // TODO: remove when we reintroduce the default fs
									},
								},
							},
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:    disk.FS_EXT4,
				BootMode:         platform.BOOT_HYBRID,
				RequiredMinSizes: map[string]datasizes.Size{"/": 3 * datasizes.GiB},
				Architecture:     arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT, // default when unspecified
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Size: 1330*datasizes.MiB + 16*datasizes.MiB + 3*datasizes.GiB + datasizes.MiB, // start + size of last partition (VG) + footer
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						UUID:     "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    1226 * datasizes.MiB,
						Size:     104 * datasizes.MiB, // the sum of the LVs (rounded to the next 4 MiB extent) grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.LVMPartitionGUID,
						UUID:     "32f3a8ae-b79e-4856-b659-c18f0dcecc77",
						Bootable: false,
						Payload: &disk.LVMVolumeGroup{
							Name:        "vg01",
							Description: "created via lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "datalv",
									Size: 100 * datasizes.MiB,
									Payload: &disk.Filesystem{
										Label:        "data",
										Type:         "ext4", // the defaultType
										Mountpoint:   "/data",
										FSTabOptions: "defaults",
										UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4",
									},
								},
							},
						},
					},
					{
						Start:    1330 * datasizes.MiB,                                                                        // the root vg is moved to the end of the partition table by relayout()
						Size:     3*datasizes.GiB + 16*datasizes.MiB + datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // the sum of the LVs (rounded to the next 4 MiB extent) grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.LVMPartitionGUID,
						UUID:     "c75e7a81-bfde-475f-a7cf-e242cf3cc354",
						Bootable: false,
						Payload: &disk.LVMVolumeGroup{
							Name:        "vg00",
							Description: "created via lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Size: 12 * datasizes.MiB, // rounded up to next extent (4 MiB)
									Payload: &disk.Filesystem{
										Label:        "var-log",
										Type:         "xfs",
										Mountpoint:   "/var/log",
										FSTabOptions: "defaults",
										UUID:         "a178892e-e285-4ce1-9114-55780875d64e",
									},
								},
								{
									Name: "rootlv",
									Size: 3 * datasizes.GiB,
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "ext4", // the defaultType
										Mountpoint:   "/",
										FSTabOptions: "defaults",
										UUID:         "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
									},
								},
							},
						},
					},
				},
			},
		},
		"btrfs-dos": {
			customizations: &blueprint.DiskCustomization{
				Type: "dos",
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "btrfs",
						MinSize: 230 * datasizes.MiB,
						BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
								},
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "subvol/varlog",
									Mountpoint: "/var/log",
								},
							},
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_EXT4,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: 1226*datasizes.MiB + 230*datasizes.MiB, // start + size of last partition + footer
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionDOSID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB, // header
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     1024 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start: 1226 * datasizes.MiB,
						Size:  230 * datasizes.MiB,
						Type:  disk.FilesystemLinuxDOSID,
						Payload: &disk.Btrfs{
							UUID: "fb180daf-48a7-4ee0-b10d-394651850fd4",
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4", // same as volume UUID
								},
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4", // same as volume UUID
								},
								{
									Name:       "subvol/varlog",
									Mountpoint: "/var/log",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4", // same as volume UUID
								},
							},
						},
					},
				},
			},
		},
		"btrfs-gpt": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "btrfs",
						MinSize: 230 * datasizes.MiB,
						BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
								},
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "subvol/varlog",
									Mountpoint: "/var/log",
								},
							},
						},
					},
					{
						MinSize: 120 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Label:  "butterswap",
							FSType: "swap",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_EXT4,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: 1346*datasizes.MiB + 230*datasizes.MiB + datasizes.MiB, // start + size of last partition + footer
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB, // header
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start:    202 * datasizes.MiB,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						UUID:     "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "boot",
							Mountpoint:   "/boot",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start: 1226 * datasizes.MiB,
						Size:  120 * datasizes.MiB,
						Type:  disk.SwapPartitionGUID,
						UUID:  "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
						Payload: &disk.Swap{
							Label:        "butterswap",
							UUID:         "fb180daf-48a7-4ee0-b10d-394651850fd4", // same as volume UUID
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    1346 * datasizes.MiB,
						Size:     231*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.FilesystemDataGUID,
						UUID:     "32f3a8ae-b79e-4856-b659-c18f0dcecc77",
						Bootable: false,
						Payload: &disk.Btrfs{
							UUID: "a178892e-e285-4ce1-9114-55780875d64e",
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
									UUID:       "a178892e-e285-4ce1-9114-55780875d64e", // same as volume UUID
								},
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
									UUID:       "a178892e-e285-4ce1-9114-55780875d64e", // same as volume UUID
								},
								{
									Name:       "subvol/varlog",
									Mountpoint: "/var/log",
									UUID:       "a178892e-e285-4ce1-9114-55780875d64e", // same as volume UUID
								},
							},
						},
					},
				},
			},
		},
		"btrfs-gpt-with-/boot": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "btrfs",
						MinSize: 230 * datasizes.MiB,
						BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
								},
								{
									Name:       "subvol/boot",
									Mountpoint: "/boot",
								},
							},
						},
					},
					{
						MinSize: 120 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Label:  "butterswap",
							FSType: "swap",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_EXT4,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: 322*datasizes.MiB + 230*datasizes.MiB + datasizes.MiB, // start + size of last partition + footer
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB, // header
						Size:     1 * datasizes.MiB,
						Bootable: true,
						Type:     disk.BIOSBootPartitionGUID,
						UUID:     disk.BIOSBootPartitionUUID,
					},
					{
						Start: 2 * datasizes.MiB, // header
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					{
						Start: 202 * datasizes.MiB,
						Size:  120 * datasizes.MiB,
						Type:  disk.SwapPartitionGUID,
						UUID:  "a178892e-e285-4ce1-9114-55780875d64e",
						Payload: &disk.Swap{
							Label:        "butterswap",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", // same as volume UUID
							FSTabOptions: "defaults",
						},
					},
					{
						Start:    322 * datasizes.MiB,
						Size:     231*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:     disk.FilesystemDataGUID,
						UUID:     "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Bootable: false,
						Payload: &disk.Btrfs{
							UUID: "fb180daf-48a7-4ee0-b10d-394651850fd4",
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4", // same as volume UUID
								},
								{
									Name:       "subvol/boot",
									Mountpoint: "/boot",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4", // same as volume UUID
								},
							},
						},
					},
				},
			},
		},
		"autorootbtrfs": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type: "btrfs",
						BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "data",
									Mountpoint: "/data",
								},
							},
						},
					},
				},
			},
			options: nil,
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: 1026 * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB,
						Size:     1024 * datasizes.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						UUID:     "a178892e-e285-4ce1-9114-55780875d64e",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "boot",
							Mountpoint:   "/boot",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start: 1025 * datasizes.MiB,
						Size:  1*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)),

						Type:     disk.FilesystemDataGUID,
						UUID:     "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Bootable: false,
						Payload: &disk.Btrfs{
							UUID: "fb180daf-48a7-4ee0-b10d-394651850fd4",
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "data",
									Mountpoint: "/data",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4",
								},
								{
									Name:       "root",
									Mountpoint: "/",
									UUID:       "fb180daf-48a7-4ee0-b10d-394651850fd4",
								},
							},
						},
					},
				},
			},
		},
		"ukiboot": {
			customizations: &blueprint.DiskCustomization{
				Type: "gpt",
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 64 * datasizes.MiB,
						// Partition type used for raw UKI UEFI binaries, more info at:
						// https://gitlab.com/CentOS/automotive/src/ukiboot/-/blob/main/README.md
						PartType:  "DF331E4D-BE00-463F-B4A7-8B43E18FB53A",
						PartLabel: "ukiboot_a",
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							FSType: "none",
						},
					},
					{
						MinSize: 64 * datasizes.MiB,
						// Partition type used for raw UKI UEFI binaries, more info at:
						// https://gitlab.com/CentOS/automotive/src/ukiboot/-/blob/main/README.md
						PartType:  "DF331E4D-BE00-463F-B4A7-8B43E18FB53A",
						PartLabel: "ukiboot_b",
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							FSType: "none",
						},
					},
					{
						MinSize: 1 * datasizes.MiB,
						// Partition type used for UKIBoot control data, more info at:
						// https://gitlab.com/CentOS/automotive/src/ukiboot/-/blob/main/README.md
						PartType:  "FEFD9070-346F-4C9A-85E6-17F07F922773",
						PartLabel: "ukibootctl",
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							FSType: "none",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				BootMode:           platform.BOOT_UEFI,
				PartitionTableType: disk.PT_GPT,
				DefaultFSType:      disk.FS_XFS,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: (1 + 200 + 64 + 64 + 1 + 1) * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					// ESP created by BOOT_UEFI option
					{
						Start: 1 * datasizes.MiB,
						Size:  200 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  disk.EFISystemPartitionUUID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         disk.EFIFilesystemUUID,
							Mountpoint:   "/boot/efi",
							Label:        "ESP",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  2,
						},
					},
					// Raw UKI partitions from customizations
					{
						Start:   201 * datasizes.MiB,
						Size:    64 * datasizes.MiB,
						Type:    "DF331E4D-BE00-463F-B4A7-8B43E18FB53A",
						Label:   "ukiboot_a",
						UUID:    "fb180daf-48a7-4ee0-b10d-394651850fd4",
						Payload: nil,
					},
					{
						Start:   265 * datasizes.MiB,
						Size:    64 * datasizes.MiB,
						Type:    "DF331E4D-BE00-463F-B4A7-8B43E18FB53A",
						Label:   "ukiboot_b",
						UUID:    "a178892e-e285-4ce1-9114-55780875d64e",
						Payload: nil,
					},
					{
						Start:   329 * datasizes.MiB,
						Size:    1 * datasizes.MiB,
						Type:    "FEFD9070-346F-4C9A-85E6-17F07F922773",
						Label:   "ukibootctl",
						UUID:    "e2d3d0d0-de6b-48f9-b44c-e85ff044c6b1",
						Payload: nil,
					},
					{
						Start: 330 * datasizes.MiB,
						Size:  1*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)), // grows by 1 grain size (1 MiB) minus the unaligned size of the header to fit the gpt footer
						Type:  disk.RootPartitionX86_64GUID,
						UUID:  "f83b8e88-3bbf-457a-ab99-c5b252c7429c",
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"gpt-blueprint-efi": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 500 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/boot/efi",
							FSType:     "vfat",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_UEFI,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_GPT,
				Size: (1 + 500 + 1) * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start: 1 * datasizes.MiB,
						Size:  500 * datasizes.MiB,
						Type:  disk.EFISystemPartitionGUID,
						UUID:  "48a79ee0-b10d-4946-9185-0fd4a178892e",
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         "6E4F-F95F",
							Mountpoint:   "/boot/efi",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    501 * datasizes.MiB,
						Size:     1*datasizes.MiB - (disk.DefaultSectorSize + (128 * 128)),
						Type:     disk.RootPartitionX86_64GUID,
						UUID:     "e285ece1-5114-4578-8875-d64ee2d3d0d0",
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "f662a5ee-e82a-4df4-8a2d-0b75fb180daf",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"dos-blueprint-efi-type": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 500 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/boot/efi",
							FSType:     "vfat",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_UEFI,
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			expected: &disk.PartitionTable{
				Type: disk.PT_DOS,
				Size: (500 + 1) * datasizes.MiB,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Partitions: []disk.Partition{
					{
						Start: 1 * datasizes.MiB,
						Size:  500 * datasizes.MiB,
						Type:  disk.EFISystemPartitionDOSID,
						Payload: &disk.Filesystem{
							Type:         "vfat",
							UUID:         "6E4F-F95F",
							Mountpoint:   "/boot/efi",
							FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    501 * datasizes.MiB,
						Size:     0 * datasizes.MiB,
						Type:     disk.FilesystemLinuxDOSID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							UUID:         "f662a5ee-e82a-4df4-8a2d-0b75fb180daf",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Initialise rng for each test separately, otherwise test run
			// order will affect results
			/* #nosec G404 */
			rnd := rand.New(rand.NewSource(0))
			pt, err := disk.NewCustomPartitionTable(tc.customizations, tc.options, rnd)

			assert.NoError(err)
			assert.Equal(tc.expected, pt)
		})
	}

}

func TestNewCustomPartitionTableSectorSize(t *testing.T) {
	type testCase struct {
		customizations *blueprint.DiskCustomization
		expectedSize   uint64
	}

	testCases := map[string]testCase{
		"default-sector-size": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 1 * datasizes.GiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							FSType:     "xfs",
						},
					},
				},
			},
			expectedSize: 0, // default, will use DefaultSectorSize
		},
		"sector-size-512": {
			customizations: &blueprint.DiskCustomization{
				SectorSize: 512,
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 1 * datasizes.GiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							FSType:     "xfs",
						},
					},
				},
			},
			expectedSize: 512,
		},
		"sector-size-4096": {
			customizations: &blueprint.DiskCustomization{
				SectorSize: 4096,
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 1 * datasizes.GiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							FSType:     "xfs",
						},
					},
				},
			},
			expectedSize: 4096,
		},
	}

	options := &disk.CustomPartitionTableOptions{
		DefaultFSType:      disk.FS_XFS,
		BootMode:           platform.BOOT_NONE,
		PartitionTableType: disk.PT_GPT,
		Architecture:       arch.ARCH_X86_64,
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			/* #nosec G404 */
			rnd := rand.New(rand.NewSource(0))
			pt, err := disk.NewCustomPartitionTable(tc.customizations, options, rnd)

			assert.NoError(err)
			assert.Equal(tc.expectedSize, pt.SectorSize, "SectorSize should match the customization")

			// Verify BytesToSectors uses the correct sector size
			effectiveSectorSize := tc.expectedSize
			if effectiveSectorSize == 0 {
				effectiveSectorSize = disk.DefaultSectorSize
			}
			assert.Equal(uint64(1024), pt.BytesToSectors(1024*effectiveSectorSize))
		})
	}
}

func TestNewCustomPartitionTableErrors(t *testing.T) {
	type testCase struct {
		customizations *blueprint.DiskCustomization
		options        *disk.CustomPartitionTableOptions
		errmsg         string
	}

	testCases := map[string]testCase{
		"autoroot-notype": {
			customizations: nil,
			options:        nil,
			errmsg:         "error generating partition table: error creating root partition: no default filesystem type",
		},
		"autorootlv-notype": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type: "lvm",
						VGCustomization: blueprint.VGCustomization{
							Name: "vg-without-root",
						},
					},
				},
			},
			options: nil,
			errmsg:  "error generating partition table: error creating root logical volume: no default filesystem type",
		},
		"notype-nodefault": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
						},
					},
				},
			},
			options: nil,
			// NOTE: this error message will change when we allow empty fs_type
			// in customizations but with a requirement to define a default
			errmsg: "error generating partition table: invalid partitioning customizations:\nunknown or invalid filesystem type (fs_type) for mountpoint \"/\": ",
		},
		"lvm-notype-nodefault": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type: "lvm",
						VGCustomization: blueprint.VGCustomization{
							Name: "rootvg",
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name: "rootlv",
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/",
									},
								},
							},
						},
					},
				},
			},
			options: nil,
			// NOTE: this error message will change when we allow empty fs_type
			// in customizations but with a requirement to define a default
			errmsg: "error generating partition table: invalid partitioning customizations:\nunknown or invalid filesystem type (fs_type) for logical volume with mountpoint \"/\": ",
		},
		"bad-pt-type": {
			options: &disk.CustomPartitionTableOptions{
				PartitionTableType: 100,
			},
			errmsg: `error generating partition table: invalid partition table type enum value: 100`,
		},
		"bad-pt-type-in-customizations": {
			customizations: &blueprint.DiskCustomization{
				Type: "toucan",
			},
			errmsg: `error generating partition table: unknown partition table type: toucan (valid: gpt, dos)`,
		},
		"dos-too-many-parts": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize: 20 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
					{
						MinSize: 5 * datasizes.MiB,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Label:  "swap",
							FSType: "swap",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			errmsg: `error generating partition table: invalid partition table: "dos" partition table type only supports up to 4 partitions: got 5 after creating the partition table with all necessary partitions`,
		},
		"bad-guid-dos": {
			customizations: &blueprint.DiskCustomization{
				Type: "dos",
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:  20 * datasizes.MiB,
						PartType: "01234567-89ab-cdef-0123-456789abcdef", // dos cannot use UUIDs
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType: disk.FS_XFS,
				BootMode:      platform.BOOT_HYBRID,
				Architecture:  arch.ARCH_X86_64,
			},
			errmsg: "error generating partition table: invalid partitioning customizations:\ninvalid partition part_type \"01234567-89ab-cdef-0123-456789abcdef\" for partition table type \"dos\" (must be a 2-digit hex number)",
		},
		"bad-guid-gpt": {
			customizations: &blueprint.DiskCustomization{
				Type: "gpt",
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:  20 * datasizes.MiB,
						PartType: "EF", // gpt requires a 36-character GUID
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType: disk.FS_XFS,
				BootMode:      platform.BOOT_HYBRID,
				Architecture:  arch.ARCH_X86_64,
			},
			errmsg: "error generating partition table: invalid partitioning customizations:\ninvalid partition part_type \"EF\" for partition table type \"gpt\" (must be a valid UUID): invalid UUID length: 2",
		},
		"bad-guid-dos-fallback": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:  20 * datasizes.MiB,
						PartType: "01234567-89ab-cdef-0123-456789abcdef", // dos cannot use UUIDs
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_DOS,
				Architecture:       arch.ARCH_X86_64,
			},
			errmsg: "error generating partition table: error validating partition type ID for \"/data\": invalid partition part_type \"01234567-89ab-cdef-0123-456789abcdef\" for partition table type \"dos\" (must be a 2-digit hex number)",
		},
		"bad-guid-gpt-fallback": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:  20 * datasizes.MiB,
						PartType: "EF", // gpt requires a 36-character GUID
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType:      disk.FS_XFS,
				BootMode:           platform.BOOT_HYBRID,
				PartitionTableType: disk.PT_GPT,
				Architecture:       arch.ARCH_X86_64,
			},
			errmsg: "error generating partition table: error validating partition type ID for \"/data\": invalid partition part_type \"EF\" for partition table type \"gpt\" (must be a valid UUID): invalid UUID length: 2",
		},
		"bad-guid-gpt-fallback-fallback": {
			customizations: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						MinSize:  20 * datasizes.MiB,
						PartType: "AA", // gpt requires a 36-character GUID
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							Label:      "data",
							FSType:     "ext4",
						},
					},
				},
			},
			options: &disk.CustomPartitionTableOptions{
				DefaultFSType: disk.FS_XFS,
				BootMode:      platform.BOOT_HYBRID,
				Architecture:  arch.ARCH_X86_64,
			},
			errmsg: "error generating partition table: error validating partition type ID for \"/data\": invalid partition part_type \"AA\" for partition table type \"gpt\" (must be a valid UUID): invalid UUID length: 2",
		},
	}

	// we don't care about the rng for error tests
	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			_, err := disk.NewCustomPartitionTable(tc.customizations, tc.options, rnd)
			assert.EqualError(err, tc.errmsg)
		})
	}
}

func TestPartitionTableFeatures(t *testing.T) {
	require := require.New(t)

	testCases := map[string]disk.PartitionTableFeatures{
		"plain":        {XFS: true, FAT: true},
		"plain-noboot": {XFS: true, FAT: true},
		"plain-swap":   {XFS: true, FAT: true, Swap: true},
		"luks":         {XFS: true, FAT: true, LUKS: true},
		"luks+lvm":     {XFS: true, FAT: true, LUKS: true, LVM: true},
		"btrfs":        {XFS: true, FAT: true, Btrfs: true},
	}

	for name, pt := range testdisk.TestPartitionTables() {
		// print an informative failure message if a new test partition
		// table is added and this test is not updated (instead of failing
		// at the final Equal() check)
		exp, ok := testCases[name]
		require.True(ok, "expected test result not defined for test partition table %q: please update the %s test", name, t.Name())
		require.Equal(exp, disk.GetPartitionTableFeatures(pt))
	}
}

func TestUnmarshalSizeUnitStringPartitionTable(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected disk.Offset
		err      error
	}{
		{
			name:     "valid size with unit",
			input:    `{"start_offset": "1 GiB", "size": "1 GiB"}`,
			expected: 1 * datasizes.GiB,
			err:      nil,
		},
		{
			name:     "valid size without unit",
			input:    `{"start_offset": 1073741824, "size": 1073741824}`,
			expected: 1 * datasizes.GiB,
			err:      nil,
		},
		{
			name:     "valid size without unit as string",
			input:    `{"start_offset": "123", "size": "123"}`,
			expected: 123,
			err:      nil,
		},
		{
			name:     "invalid size with unit",
			input:    `{"start_offset": "1 GGB"}`,
			expected: 0,
			err:      fmt.Errorf("unknown data size units in string: 1 GGB"),
		},
		{
			name:     "invalid size with unit",
			input:    `{"size": "1 GGB"}`,
			expected: 0,
			err:      fmt.Errorf("error decoding size: unknown data size units in string: 1 GGB"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pt disk.PartitionTable
			err := json.Unmarshal([]byte(tc.input), &pt)
			if tc.err != nil {
				assert.ErrorContains(t, err, tc.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, pt.StartOffset)
			assert.Equal(t, datasizes.Size(tc.expected), pt.Size)
		})
	}
}
