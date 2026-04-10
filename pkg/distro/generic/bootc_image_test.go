package generic

import (
	"math/rand"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/disk/partition"
	"github.com/osbuild/images/pkg/distro"
	"github.com/stretchr/testify/assert"
)

func createRand() *rand.Rand {
	return rand.New(rand.NewSource(0))
}

func TestCheckFilesystemCustomizationsValidates(t *testing.T) {
	for _, tc := range []struct {
		fsCust      []blueprint.FilesystemCustomization
		ptmode      partition.PartitioningMode
		expectedErr string
	}{
		// happy
		{
			fsCust:      []blueprint.FilesystemCustomization{},
			expectedErr: "",
		},
		{
			fsCust:      []blueprint.FilesystemCustomization{},
			ptmode:      partition.BtrfsPartitioningMode,
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"}, {Mountpoint: "/boot"},
			},
			ptmode:      partition.RawPartitioningMode,
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"}, {Mountpoint: "/boot"},
			},
			ptmode:      partition.BtrfsPartitioningMode,
			expectedErr: "",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/boot"},
				{Mountpoint: "/var/log"},
				{Mountpoint: "/var/data"},
			},
			expectedErr: "",
		},
		// sad
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/ostree"},
			},
			ptmode:      partition.RawPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/ostree\" is not allowed",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/var"},
			},
			ptmode:      partition.RawPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/var\" is not allowed",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/var/data"},
			},
			ptmode:      partition.BtrfsPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/var/data\" is not allowed",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/boot/"},
			},
			ptmode:      partition.BtrfsPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/boot/\" must be canonical",
		},
		{
			fsCust: []blueprint.FilesystemCustomization{
				{Mountpoint: "/"},
				{Mountpoint: "/boot/"},
				{Mountpoint: "/opt"},
			},
			ptmode:      partition.BtrfsPartitioningMode,
			expectedErr: "the following errors occurred while validating custom mountpoints:\npath \"/boot/\" must be canonical\npath \"/opt\" is not allowed",
		},
	} {
		if tc.expectedErr == "" {
			assert.NoError(t, checkFilesystemCustomizations(tc.fsCust, tc.ptmode))
		} else {
			assert.ErrorContains(t, checkFilesystemCustomizations(tc.fsCust, tc.ptmode), tc.expectedErr)
		}
	}
}

func TestLocalMountpointPolicy(t *testing.T) {
	// extended testing of the general mountpoint policy (non-minimal)
	type testCase struct {
		path    string
		allowed bool
	}

	testCases := []testCase{
		// existing mountpoints / and /boot are fine for sizing
		{"/", true},
		{"/boot", true},

		// root mountpoints are not allowed
		{"/data", false},
		{"/opt", false},
		{"/stuff", false},
		{"/usr", false},

		// /var explicitly is not allowed
		{"/var", false},

		// subdirs of /boot are not allowed
		{"/boot/stuff", false},
		{"/boot/loader", false},

		// /var subdirectories are allowed
		{"/var/data", true},
		{"/var/scratch", true},
		{"/var/log", true},
		{"/var/opt", true},
		{"/var/opt/application", true},

		// but not these
		{"/var/home", false},
		{"/var/lock", false}, // symlink to ../run/lock which is on tmpfs
		{"/var/mail", false}, // symlink to spool/mail
		{"/var/mnt", false},
		{"/var/roothome", false},
		{"/var/run", false}, // symlink to ../run which is on tmpfs
		{"/var/srv", false},
		{"/var/usrlocal", false},

		// nor their subdirs
		{"/var/run/subrun", false},
		{"/var/srv/test", false},
		{"/var/home/user", false},
		{"/var/usrlocal/bin", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			err := checkFilesystemCustomizations([]blueprint.FilesystemCustomization{{Mountpoint: tc.path}}, partition.RawPartitioningMode)
			if err != nil && tc.allowed {
				t.Errorf("expected %s to be allowed, but got error: %v", tc.path, err)
			} else if err == nil && !tc.allowed {
				t.Errorf("expected %s to be denied, but got no error", tc.path)
			}
		})
	}
}

func TestUpdateFilesystemSizes(t *testing.T) {
	type testCase struct {
		customizations []blueprint.FilesystemCustomization
		minRootSize    uint64
		expected       []blueprint.FilesystemCustomization
	}

	testCases := map[string]testCase{
		"simple": {
			customizations: nil,
			minRootSize:    999,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    999,
				},
			},
		},
		"container-is-larger": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    10,
				},
			},
			minRootSize: 999,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    999,
				},
			},
		},
		"container-is-smaller": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    1000,
				},
			},
			minRootSize: 892,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    1000,
				},
			},
		},
		"customizations-noroot": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
			},
			minRootSize: 9000,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    9000,
				},
			},
		},
		"customizations-withroot-smallcontainer": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    2_000_000,
				},
			},
			minRootSize: 9000,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    2_000_000,
				},
			},
		},
		"customizations-withroot-largecontainer": {
			customizations: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    2_000_000,
				},
			},
			minRootSize: 9_000_000,
			expected: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/var/data",
					MinSize:    1_000_000,
				},
				{
					Mountpoint: "/",
					MinSize:    9_000_000,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.ElementsMatch(t, updateFilesystemSizes(tc.customizations, tc.minRootSize), tc.expected)
		})
	}

}

func findMountableSizeableFor(pt *disk.PartitionTable, needle string) (disk.Mountable, disk.Sizeable) {
	var foundMnt disk.Mountable
	var foundParent disk.Sizeable
	err := pt.ForEachMountable(func(mnt disk.Mountable, path []disk.Entity) error {
		if mnt.GetMountpoint() == needle {
			foundMnt = mnt
			for idx := len(path) - 1; idx >= 0; idx-- {
				if sz, ok := path[idx].(disk.Sizeable); ok {
					foundParent = sz
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return foundMnt, foundParent
}

func TestGenPartitionTableSetsRootfsForAllFilesystemsXFS(t *testing.T) {
	rng := createRand()

	imgType := NewTestBootcImageType(t, "qcow2")

	cus := &blueprint.Customizations{
		Filesystem: []blueprint.FilesystemCustomization{
			{Mountpoint: "/var/data", MinSize: 2_000_000},
			{Mountpoint: "/var/stuff", MinSize: 10_000_000},
		},
	}
	rootfsMinSize := uint64(0)
	pt, err := imgType.genPartitionTable(cus, rootfsMinSize, rng)
	assert.NoError(t, err)

	for _, mntPoint := range []string{"/", "/boot", "/var/data"} {
		mnt, _ := findMountableSizeableFor(pt, mntPoint)
		assert.Equal(t, "xfs", mnt.GetFSType())
	}
	_, parent := findMountableSizeableFor(pt, "/var/data")
	assert.True(t, parent.GetSize() >= 2_000_000)

	_, parent = findMountableSizeableFor(pt, "/var/stuff")
	assert.True(t, parent.GetSize() >= 10_000_000)

	// ESP is always vfat
	mnt, _ := findMountableSizeableFor(pt, "/boot/efi")
	assert.Equal(t, "vfat", mnt.GetFSType())
}

func TestGenPartitionTableSetsRootfsForAllFilesystemsBtrfs(t *testing.T) {
	rng := createRand()

	d := NewTestBootcDistro(t)
	d.defaultFs = "btrfs"
	it, err := common.Must(d.GetArch("x86_64")).GetImageType("qcow2")
	assert.NoError(t, err)
	imgType := it.(*bootcImageType)
	cus := &blueprint.Customizations{}
	rootfsMinSize := uint64(0)
	pt, err := imgType.genPartitionTable(cus, rootfsMinSize, rng)
	assert.NoError(t, err)

	mnt, _ := findMountableSizeableFor(pt, "/")
	assert.Equal(t, "btrfs", mnt.GetFSType())

	// btrfs has a default (xfs) /boot
	mnt, _ = findMountableSizeableFor(pt, "/boot")
	assert.Equal(t, "xfs", mnt.GetFSType())

	// ESP is always vfat
	mnt, _ = findMountableSizeableFor(pt, "/boot/efi")
	assert.Equal(t, "vfat", mnt.GetFSType())
}
func TestGenPartitionTableDiskCustomizationRunsValidateLayoutConstraints(t *testing.T) {
	rng := createRand()

	imgType := NewTestBootcImageType(t, "qcow2")

	cus := &blueprint.Customizations{
		Disk: &blueprint.DiskCustomization{
			Partitions: []blueprint.PartitionCustomization{
				{
					Type:            "lvm",
					VGCustomization: blueprint.VGCustomization{},
				},
				{
					Type:            "lvm",
					VGCustomization: blueprint.VGCustomization{},
				},
			},
		},
	}
	_, err := imgType.genPartitionTable(cus, 0, rng)
	assert.EqualError(t, err, "cannot use disk customization: multiple LVM volume groups are not yet supported")
}

func TestGenPartitionTableDiskCustomizationUnknownTypesError(t *testing.T) {
	cus := &blueprint.Customizations{
		Disk: &blueprint.DiskCustomization{
			Partitions: []blueprint.PartitionCustomization{
				{
					Type: "rando",
				},
			},
		},
	}
	_, err := calcRequiredDirectorySizes(cus.Disk, 5*datasizes.GiB)
	assert.EqualError(t, err, `unknown disk customization type "rando"`)
}

func TestGenPartitionTableDiskCustomizationSizes(t *testing.T) {
	rng := createRand()

	for _, tc := range []struct {
		name                string
		rootfsMinSize       uint64
		partitions          []blueprint.PartitionCustomization
		expectedMinRootSize datasizes.Size
	}{
		{
			"empty disk customizaton, root expands to rootfsMinsize",
			2 * datasizes.GiB,
			nil,
			2 * datasizes.GiB,
		},
		// plain
		{
			"plain, no root minsize, expands to rootfsMinSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					MinSize: 10 * datasizes.GiB,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/var",
						FSType:     "xfs",
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"plain, small root minsize, expands to rootfsMnSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					MinSize: 1 * datasizes.GiB,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/",
						FSType:     "xfs",
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"plain, big root minsize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					MinSize: 10 * datasizes.GiB,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/",
						FSType:     "xfs",
					},
				},
			},
			10 * datasizes.GiB,
		},
		// btrfs
		{
			"btrfs, no root minsize, expands to rootfsMinSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "btrfs",
					MinSize: 10 * datasizes.GiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Mountpoint: "/var",
								Name:       "varvol",
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"btrfs, small root minsize, expands to rootfsMnSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "btrfs",
					MinSize: 1 * datasizes.GiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Mountpoint: "/",
								Name:       "rootvol",
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"btrfs, big root minsize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "btrfs",
					MinSize: 10 * datasizes.GiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Mountpoint: "/",
								Name:       "rootvol",
							},
						},
					},
				},
			},
			10 * datasizes.GiB,
		},
		// lvm
		{
			"lvm, no root minsize, expands to rootfsMinSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "lvm",
					MinSize: 10 * datasizes.GiB,
					VGCustomization: blueprint.VGCustomization{
						LogicalVolumes: []blueprint.LVCustomization{
							{
								MinSize: 10 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/var",
									FSType:     "xfs",
								},
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"lvm, small root minsize, expands to rootfsMnSize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "lvm",
					MinSize: 1 * datasizes.GiB,
					VGCustomization: blueprint.VGCustomization{
						LogicalVolumes: []blueprint.LVCustomization{
							{
								MinSize: 1 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									FSType:     "xfs",
								},
							},
						},
					},
				},
			},
			5 * datasizes.GiB,
		},
		{
			"lvm, big root minsize",
			5 * datasizes.GiB,
			[]blueprint.PartitionCustomization{
				{
					Type:    "lvm",
					MinSize: 10 * datasizes.GiB,
					VGCustomization: blueprint.VGCustomization{
						LogicalVolumes: []blueprint.LVCustomization{
							{
								MinSize: 10 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									FSType:     "xfs",
								},
							},
						},
					},
				},
			},
			10 * datasizes.GiB,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			imgType := NewTestBootcImageType(t, "qcow2")

			rootfsMinsize := tc.rootfsMinSize
			cus := &blueprint.Customizations{
				Disk: &blueprint.DiskCustomization{
					Partitions: tc.partitions,
				},
			}
			pt, err := imgType.genPartitionTable(cus, rootfsMinsize, rng)
			assert.NoError(t, err)

			var rootSize datasizes.Size
			err = pt.ForEachMountable(func(mnt disk.Mountable, path []disk.Entity) error {
				if mnt.GetMountpoint() == "/" {
					for idx := len(path) - 1; idx >= 0; idx-- {
						if parent, ok := path[idx].(disk.Sizeable); ok {
							rootSize = parent.GetSize()
							break
						}
					}
				}
				return nil
			})
			assert.NoError(t, err)
			// expected size is within a reasonable limit
			assert.True(t, rootSize >= tc.expectedMinRootSize && rootSize < tc.expectedMinRootSize+5*datasizes.MiB)
		})
	}
}

func TestManifestFilecustomizationsSad(t *testing.T) {
	imgType := NewTestBootcImageType(t, "qcow2")
	bp := &blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Files: []blueprint.FileCustomization{
				{
					Path: "/not/allowed",
					Data: "some-data",
				},
			},
		},
	}

	_, _, err := imgType.Manifest(bp, distro.ImageOptions{}, nil, common.ToPtr(int64(0)))
	assert.EqualError(t, err, `the following custom files are not allowed: ["/not/allowed"]`)
}

func TestManifestDirCustomizationsSad(t *testing.T) {
	imgType := NewTestBootcImageType(t, "qcow2")
	bp := &blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Directories: []blueprint.DirectoryCustomization{
				{
					Path: "/dir/not/allowed",
				},
			},
		},
	}

	_, _, err := imgType.Manifest(bp, distro.ImageOptions{}, nil, common.ToPtr(int64(0)))
	assert.EqualError(t, err, `the following custom directories are not allowed: ["/dir/not/allowed"]`)
}

func TestGenPartitionTableFromOSInfo(t *testing.T) {
	var bp blueprint.Blueprint
	imgType := NewTestBootcImageType(t, "qcow2")
	// pretend a custom partition table is set via the bootc
	// container sourceInfo mechanism
	newPt, err := imgType.BasePartitionTable()
	assert.NoError(t, err)
	newPt.UUID = "01010101-01011-01011-01011-01010101"
	d := imgType.arch.distro.(*BootcDistro)
	d.sourceInfo.PartitionTable = newPt

	// validate that the container uuid is part of the generated
	// manifest
	mf, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, common.ToPtr(int64(0)))
	assert.NoError(t, err)
	manifestJson, err := mf.Serialize(nil, diskContainers, nil, nil, nil)
	assert.NoError(t, err)
	assert.Contains(t, string(manifestJson), "01010101-01011-01011-01011-01010101")
}
