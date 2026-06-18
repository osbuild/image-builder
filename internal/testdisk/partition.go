package testdisk

import (
	"math/rand"

	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/osbuild/image-builder/pkg/disk"
)

const (
	KiB = datasizes.KiB
	MiB = datasizes.MiB
	GiB = datasizes.GiB
)

const FakePartitionSize = datasizes.Size(789 * MiB)

// TODO: Tidy up and unify TestPartitionTables with the fake partition table
// generators below (MakeFake*). Maybe use NewCustomPartitionTable() to
// generate test partition tables instead.

func TestPartitionTables() map[string]disk.PartitionTable {
	return map[string]disk.PartitionTable{
		"plain": {
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * MiB,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 200 * MiB,
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
				{
					Size: 500 * MiB,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		},

		"plain-swap": {
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * MiB,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 200 * MiB,
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
				{
					Size: 500 * MiB,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Size: 512 * MiB,
					Type: disk.SwapPartitionGUID,
					Payload: &disk.Swap{
						Label:        "swap",
						FSTabOptions: "defaults",
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		},

		"plain-noboot": {
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * MiB,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 200 * MiB,
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
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		},

		"luks": {
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * MiB,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 200 * MiB,
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
				{
					Size: 500 * MiB,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.LUKSContainer{
						UUID:  "",
						Label: "crypt_root",
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"luks+lvm": {
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * MiB,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 200 * MiB,
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
				{
					Size: 500 * MiB,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Size: 5 * GiB,
					Payload: &disk.LUKSContainer{
						UUID: "",
						Payload: &disk.LVMVolumeGroup{
							Name:        "",
							Description: "",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Size: 2 * GiB,
									Payload: &disk.Filesystem{
										Type:         "xfs",
										Label:        "root",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
										FSTabFreq:    0,
										FSTabPassNo:  0,
									},
								},
								{
									Size: 2 * GiB,
									Payload: &disk.Filesystem{
										Type:         "xfs",
										Label:        "root",
										Mountpoint:   "/home",
										FSTabOptions: "defaults",
										FSTabFreq:    0,
										FSTabPassNo:  0,
									},
								},
							},
						},
					},
				},
			},
		},
		"btrfs": {
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * MiB,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 200 * MiB,
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
				{
					Size: 500 * MiB,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Size: 10 * GiB,
					Payload: &disk.Btrfs{
						UUID:       "",
						Label:      "",
						Mountpoint: "",
						Subvolumes: []disk.BtrfsSubvolume{
							{
								Name:       "root",
								Size:       0,
								Mountpoint: "/",
								GroupID:    0,
							},
							{
								Name:       "var",
								Size:       5 * GiB,
								Mountpoint: "/var",
								GroupID:    0,
							},
						},
					},
				},
			},
		},
	}
}

// MakeFakePartitionTable is a helper to create partition table structs
// for tests. It uses sensible defaults for common scenarios.
// Including a "swap" entry creates a swap partition.
func MakeFakePartitionTable(mntPoints ...string) *disk.PartitionTable {
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(0))

	var partitions []disk.Partition
	for _, mntPoint := range mntPoints {
		var payload disk.PayloadEntity
		switch mntPoint {
		case "/":
			payload = &disk.Filesystem{
				Type:       "ext4",
				Mountpoint: mntPoint,
				UUID:       disk.RootPartitionUUID,
			}
		case "/boot/efi":
			payload = &disk.Filesystem{
				Type:       "vfat",
				Mountpoint: mntPoint,
				UUID:       disk.EFIFilesystemUUID,
			}
		case "swap":
			swap := &disk.Swap{
				Label: "swap",
			}
			swap.GenUUID(rng)
			payload = swap
		case "raw":
			payload = &disk.Raw{
				SourcePath: "/usr/lib/modules/5.0/aboot.img",
			}
		default:
			payload = &disk.Filesystem{
				Type:       "ext4",
				Mountpoint: mntPoint,
				UUID:       disk.DataPartitionUUID,
			}
		}
		partitions = append(partitions, disk.Partition{
			Size:    FakePartitionSize,
			Payload: payload,
		})

	}
	return &disk.PartitionTable{
		Type:       disk.PT_GPT,
		Partitions: partitions,
	}
}

// MakeFakeBtrfsPartitionTable is similar to MakeFakePartitionTable but
// creates a btrfs-based partition table.
// Including a "swap" entry creates a swap partition.
func MakeFakeBtrfsPartitionTable(mntPoints ...string) *disk.PartitionTable {
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(0))

	var subvolumes []disk.BtrfsSubvolume
	pt := &disk.PartitionTable{
		Type:       disk.PT_GPT,
		Size:       10 * GiB,
		Partitions: []disk.Partition{},
	}
	size := uint64(0)
	for _, mntPoint := range mntPoints {
		switch mntPoint {
		case "/boot":
			pt.Partitions = append(pt.Partitions, disk.Partition{
				Start: size,
				Size:  1 * GiB,
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: mntPoint,
				},
			})
			size += 1 * GiB
		case "/boot/efi":
			pt.Partitions = append(pt.Partitions, disk.Partition{
				Start: size,
				Size:  100 * MiB,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: mntPoint,
					UUID:       disk.EFIFilesystemUUID,
				},
			})
			size += 100 * MiB
		case "swap":
			swap := &disk.Swap{
				Label: "swap",
			}
			swap.GenUUID(rng)
			pt.Partitions = append(pt.Partitions, disk.Partition{
				Start:   size,
				Size:    512 * MiB,
				Payload: swap,
			})
			size += 512 * MiB
		default:
			name := mntPoint
			uuid := disk.RootPartitionUUID
			if name == "/" {
				name = "root"
			}
			subvolumes = append(
				subvolumes,
				disk.BtrfsSubvolume{
					Mountpoint: mntPoint,
					Name:       name,
					UUID:       uuid,
					Compress:   disk.DefaultBtrfsCompression,
				},
			)
		}
	}

	pt.Partitions = append(pt.Partitions, disk.Partition{
		Start: size,
		Size:  9 * GiB,
		Payload: &disk.Btrfs{
			UUID:       disk.RootPartitionUUID,
			Subvolumes: subvolumes,
		},
	})

	size += 9 * GiB
	pt.Size = datasizes.Size(size)

	return pt
}

// MakeFakeLVMPartitionTable is similar to MakeFakePartitionTable but
// creates a lvm-based partition table.
// Including a "swap" entry creates a swap logical volume.
func MakeFakeLVMPartitionTable(mntPoints ...string) *disk.PartitionTable {
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(0))

	var lvs []disk.LVMLogicalVolume
	pt := &disk.PartitionTable{
		Type:       disk.PT_GPT,
		Size:       10 * GiB,
		Partitions: []disk.Partition{},
	}
	size := uint64(0)
	for _, mntPoint := range mntPoints {
		switch mntPoint {
		case "/boot":
			pt.Partitions = append(pt.Partitions, disk.Partition{
				Start: size,
				Size:  1 * GiB,
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: mntPoint,
				},
			})
			size += 1 * GiB
		case "/boot/efi":
			pt.Partitions = append(pt.Partitions, disk.Partition{
				Start: size,
				Size:  100 * MiB,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: mntPoint,
					UUID:       disk.EFIFilesystemUUID,
				},
			})
			size += 100 * MiB
		case "swap":
			swap := &disk.Swap{
				Label: "swap",
			}
			swap.GenUUID(rng)
			lvs = append(
				lvs,
				disk.LVMLogicalVolume{
					Name:    "lv-for-swap",
					Payload: swap,
				},
			)
		default:
			name := "lv-for-" + mntPoint
			if name == "/" {
				name = "lvroot"
			}
			lvs = append(
				lvs,
				disk.LVMLogicalVolume{
					Name: name,
					Payload: &disk.Filesystem{
						Type:       "xfs",
						Mountpoint: mntPoint,
					},
				},
			)
		}
	}

	pt.Partitions = append(pt.Partitions, disk.Partition{
		Start: size,
		Size:  9 * GiB,
		Payload: &disk.LVMVolumeGroup{
			Name:           "rootvg",
			LogicalVolumes: lvs,
		},
	})

	size += 9 * GiB
	pt.Size = datasizes.Size(size)

	return pt
}
