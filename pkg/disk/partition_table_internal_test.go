package disk

import (
	"fmt"
	"testing"

	"github.com/osbuild/images/pkg/datasizes"
	"github.com/stretchr/testify/assert"
)

const (
	KiB = datasizes.KiB
	MiB = datasizes.MiB
	GiB = datasizes.GiB
)

// validatePTSize checks that each Partition is large enough to contain every
// sizeable under it.
func validatePTSize(pt *PartitionTable) error {
	ptTotal := datasizes.Size(0)
	for _, partition := range pt.Partitions {
		if err := validateEntitySize(&partition, partition.GetSize()); err != nil {
			return err
		}
		ptTotal += partition.GetSize()
	}

	if pt.GetSize() < ptTotal {
		return fmt.Errorf("PartitionTable size %d is smaller than the sum of its partitions %d", pt.GetSize(), ptTotal)
	}
	return nil
}

// validateEntitySize checks that every sizeable under a given Entity can be
// contained in the given size.
func validateEntitySize(ent Entity, size datasizes.Size) error {
	if cont, ok := ent.(Container); ok {
		containerTotal := datasizes.Size(0)
		for idx := uint(0); idx < cont.GetItemCount(); idx++ {
			child := cont.GetChild(idx)
			var childSize datasizes.Size
			if sizeable, convOk := child.(Sizeable); convOk {
				childSize = sizeable.GetSize()
				containerTotal += childSize
			} else {
				// child is not sizeable: use the parent size
				childSize = size
			}
			if err := validateEntitySize(child, childSize); err != nil {
				return err
			}
		}

		if size < containerTotal {
			return fmt.Errorf("Entity size %d is smaller than the sum of its children %d", size, containerTotal)
		}
	}
	// non-containers need no checking
	return nil
}

func TestValidateFunctions(t *testing.T) {
	type testCase struct {
		pt  *PartitionTable
		err error
	}

	testCases := map[string]testCase{
		"happy-simple": {
			pt: &PartitionTable{
				Size: 100,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
					},
				},
			},
			err: nil,
		},
		"happy-nested": {
			pt: &PartitionTable{
				Size: 100,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 5,
								},
								{
									Size: 8,
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		"happy-btrfs": {
			pt: &PartitionTable{
				Size: 100,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
						Payload: &Btrfs{
							Subvolumes: []BtrfsSubvolume{
								{
									Size: 4,
								},
								{
									Size: 2,
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		"unhappy-simple": {
			pt: &PartitionTable{
				Size: 10,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
					},
				},
			},
			err: fmt.Errorf("PartitionTable size 10 is smaller than the sum of its partitions 30"),
		},
		"unhappy-nested": {
			pt: &PartitionTable{
				Size: 100,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 15,
								},
								{
									Size: 8,
								},
							},
						},
					},
				},
			},
			err: fmt.Errorf("Entity size 20 is smaller than the sum of its children 23"),
		},
		"unhappy-nested-luks": {
			pt: &PartitionTable{
				Size: 100,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
						Payload: &LUKSContainer{
							Payload: &LVMVolumeGroup{
								LogicalVolumes: []LVMLogicalVolume{
									{
										Size: 15,
									},
									{
										Size: 8,
									},
								},
							},
						},
					},
				},
			},
			err: fmt.Errorf("Entity size 20 is smaller than the sum of its children 23"),
		},
		"unhappy-btrfs": {
			pt: &PartitionTable{
				Size: 100,
				Partitions: []Partition{
					{
						Size: 10,
					},
					{
						Size: 20,
						Payload: &Btrfs{
							Subvolumes: []BtrfsSubvolume{
								{
									Size: 10,
								},
								{
									Size: 10,
								},
								{
									Size: 1,
								},
							},
						},
					},
				},
			},
			err: fmt.Errorf("Entity size 20 is smaller than the sum of its children 21"),
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			err := validatePTSize(tc.pt)
			assert.Equal(t, tc.err, err)
		})
	}
}

func TestRelayout(t *testing.T) {
	type testCase struct {
		pt       *PartitionTable
		size     datasizes.Size
		expected *PartitionTable
	}

	testCases := map[string]testCase{
		"simple-dos": {
			pt: &PartitionTable{
				Type: PT_DOS,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Size: 20 * MiB,
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_DOS,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Start: 11 * MiB,
						Size:  89 * MiB, // Grows to fill the space
					},
				},
			},
		},
		"simple-gpt": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Size: 20 * MiB,
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // header (1 sector + 128 B * 128 partitions) aligned up to the default grain (1 MiB)
						Size:  10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Start: 11 * MiB,
						Size:  89*MiB - (DefaultSectorSize + (128 * 128)), // Grows to fill the space, but gpt adds a footer the same size as the header (unaligned)
					},
				},
			},
		},
		"simple-gpt-root-first": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 10 * MiB,
						Payload: &Filesystem{
							Mountpoint: "/",
						},
					},
					{
						Size: 20 * MiB,
					},
					{
						Size: 30 * MiB,
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // header (1 sector + 128 B * 128 partitions) aligned up to the default grain (1 MiB)
						Size:  20 * MiB,
					},
					{
						Start: 21 * MiB, // header (1 sector + 128 B * 128 partitions) aligned up to the default grain (1 MiB)
						Size:  30 * MiB,
					},
					{
						Start: 51 * MiB,                                   // root gets moved to last position
						Size:  49*MiB - (DefaultSectorSize + (128 * 128)), // Grows to fill the space, but gpt adds a footer the same size as the header (unaligned)
						Payload: &Filesystem{
							Mountpoint: "/",
						},
					},
				},
			},
		},
		"lvm-dos": {
			pt: &PartitionTable{
				Type: PT_DOS,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 20 * MiB,
					},
					{
						Size: 30 * MiB,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
								},
							},
						},
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_DOS,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  20 * MiB,
					},
					{
						Start: 21 * MiB,
						Size:  79 * MiB, // Grows to fill the space
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
								},
							},
						},
					},
				},
			},
		},
		"lvm-gpt": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 20 * MiB,
					},
					{
						Size: 30 * MiB,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 10 * MiB,
								},
							},
						},
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  20 * MiB,
					},
					{
						Start: 21 * MiB,
						Size:  79*MiB - (DefaultSectorSize + (128 * 128)), // Grows to fill the space, but gpt adds a footer the same size as the header (unaligned)
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 10 * MiB, // We don't automatically grow the root LV
								},
							},
						},
					},
				},
			},
		},
		"lvm-gpt-multilv": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 20 * MiB,
					},
					{
						Size: 30 * MiB,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 10 * MiB,
								},
							},
						},
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  20 * MiB,
					},
					{
						Start: 21 * MiB,
						Size:  79*MiB - (DefaultSectorSize + (128 * 128)), // Grows to fill the space, but gpt adds a footer the same size as the header (unaligned)
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 10 * MiB, // We don't automatically grow the root LV
								},
							},
						},
					},
				},
			},
		},
		"btrfs": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 20 * MiB,
					},
					{
						Size: 30 * MiB,
						Payload: &Btrfs{
							Subvolumes: []BtrfsSubvolume{
								{
									Size: 20 * MiB,
								},
								{
									Mountpoint: "/",
									Size:       10 * MiB,
								},
							},
						},
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  20 * MiB,
					},
					{
						Start: 21 * MiB,
						Size:  79*MiB - (DefaultSectorSize + (128 * 128)), // Grows to fill the space, but gpt adds a footer the same size as the header (unaligned)
						Payload: &Btrfs{
							Subvolumes: []BtrfsSubvolume{
								{
									Size: 20 * MiB,
								},
								{
									Mountpoint: "/",
									Size:       10 * MiB, // We don't automatically grow the root subvolume
								},
							},
						},
					},
				},
			},
		},
		"simple-dos-grow-pt": {
			pt: &PartitionTable{
				Type: PT_DOS,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Size: 200 * MiB,
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_DOS,
				Size: 211 * MiB, // grows to fit partitions and header
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Start: 11 * MiB,
						Size:  200 * MiB,
					},
				},
			},
		},
		"simple-gpt-growpt": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 100 * MiB,
				Partitions: []Partition{
					{
						Size: 10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Size: 500 * MiB,
					},
				},
			},
			size: 42 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 512 * MiB, // grows to fit partitions, header, and footer
				Partitions: []Partition{
					{
						Start: 1 * MiB, // header (1 sector + 128 B * 128 partitions) aligned up to the default grain (1 MiB)
						Size:  10 * MiB,
					},
					{
						Payload: &Filesystem{
							Mountpoint: "/",
						},
						Start: 11 * MiB,
						Size:  501*MiB - (DefaultSectorSize + (128 * 128)), // grows by (1 MiB - footer) so that the partition doesn't shrink below the desired root size
					},
				},
			},
		},
		"lvm-gpt-grow": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 10 * MiB,
				Partitions: []Partition{
					{
						Size: 200 * MiB,
					},
					{
						Size: 500 * MiB,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 10 * MiB,
								},
							},
						},
					},
				},
			},
			size: 100 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 702 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  200 * MiB,
					},
					{
						Start: 201 * MiB,
						Size:  501*MiB - (DefaultSectorSize + (128 * 128)), // grows by (1 MiB - footer) so that the partition doesn't shrink below the desired root size
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 10 * MiB, // We don't automatically grow the root LV
								},
							},
						},
					},
				},
			},
		},
		"lvm-dos-grow-rootvg": {
			pt: &PartitionTable{
				Type: PT_DOS,
				Size: 10 * MiB, // PT is smaller than the sum of Partitions
				Partitions: []Partition{
					{
						Size: 200 * MiB,
					},
					{
						Size: 10 * MiB, // VG partition is smaller than sum of LVs
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 100 * MiB,
								},
							},
						},
					},
				},
			},
			size: 99 * MiB,
			expected: &PartitionTable{
				Type: PT_DOS,
				Size: 325 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  200 * MiB,
					},
					{
						Start: 201 * MiB,
						Size:  124 * MiB, // grows to fit logical volumes + 1 MiB metadata, rounded up to default extent size (4 MiB)
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 100 * MiB,
								},
							},
						},
					},
				},
			},
		},
		"lvm-gpt-grow-rootvg": {
			pt: &PartitionTable{
				Type: PT_GPT,
				Size: 10 * MiB,
				Partitions: []Partition{
					{
						Size: 200 * MiB,
					},
					{
						Size: 10 * MiB,
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 100 * MiB,
								},
							},
						},
					},
				},
			},
			size: 99 * MiB,
			expected: &PartitionTable{
				Type: PT_GPT,
				Size: 326 * MiB,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // 1 sector header aligned up to the default grain (1 MiB)
						Size:  200 * MiB,
					},
					{
						Start: 201 * MiB,
						Size:  125*MiB - (DefaultSectorSize + (128 * 128)), // grows to fit logical volumes and metadata, rounded up to default extent size + (1 MiB - footer) so that the no partitions shrink below the desired sizes
						Payload: &LVMVolumeGroup{
							LogicalVolumes: []LVMLogicalVolume{
								{
									Size: 20 * MiB,
								},
								{
									Payload: &Filesystem{
										Mountpoint: "/",
									},
									Size: 100 * MiB,
								},
							},
						},
					},
				},
			},
		},
		"gpt-4k-grain-align-footer": {
			pt: &PartitionTable{
				Type:        PT_GPT,
				Size:        200*MiB + 2*GiB,
				GrainSize:   4096,
				AlignFooter: true,
				Partitions: []Partition{
					{
						Size: 200 * MiB,
						Type: EFISystemPartitionGUID,
						Payload: &Filesystem{
							Type:       "vfat",
							Mountpoint: "/boot/efi",
						},
					},
					{
						Size: 2 * GiB,
						Payload: &Filesystem{
							Mountpoint: "/",
						},
					},
				},
			},
			size: 200*MiB + 2*GiB,
			expected: &PartitionTable{
				Type:        PT_GPT,
				Size:        200*MiB + 2*GiB + 10*4096, // header (5*4096) + footer (5*4096)
				GrainSize:   4096,
				AlignFooter: true,
				Partitions: []Partition{
					{
						Start: 5 * 4096, // header (16896) aligned up to 4096 grain
						Size:  200 * MiB,
						Type:  EFISystemPartitionGUID,
						Payload: &Filesystem{
							Type:       "vfat",
							Mountpoint: "/boot/efi",
						},
					},
					{
						Start: 5*4096 + 200*MiB,
						Size:  2 * GiB, // exactly 2 GiB: footer is grain-aligned so root stays aligned
						Payload: &Filesystem{
							Mountpoint: "/",
						},
					},
				},
			},
		},
		"gpt-4k-grain-repart-compat": {
			pt: &PartitionTable{
				Type:                PT_GPT,
				Size:                200*MiB + 2*GiB,
				GrainSize:           4096,
				StartOffset:         1 * MiB,
				AbsoluteStartOffset: true,
				AlignFooter:         true,
				Partitions: []Partition{
					{
						Size: 200 * MiB,
						Type: EFISystemPartitionGUID,
						Payload: &Filesystem{
							Type:       "vfat",
							Mountpoint: "/boot/efi",
						},
					},
					{
						Size: 2 * GiB,
						Payload: &Filesystem{
							Mountpoint: "/",
						},
					},
				},
			},
			size: 200*MiB + 2*GiB,
			expected: &PartitionTable{
				Type:                PT_GPT,
				Size:                1*MiB + 200*MiB + 2*GiB + 5*4096, // start (1 MiB) + partitions + aligned footer
				GrainSize:           4096,
				StartOffset:         1 * MiB,
				AbsoluteStartOffset: true,
				AlignFooter:         true,
				Partitions: []Partition{
					{
						Start: 1 * MiB, // StartOffset treated as absolute minimum, already grain-aligned
						Size:  200 * MiB,
						Type:  EFISystemPartitionGUID,
						Payload: &Filesystem{
							Type:       "vfat",
							Mountpoint: "/boot/efi",
						},
					},
					{
						Start: 1*MiB + 200*MiB,
						Size:  2 * GiB,
						Payload: &Filesystem{
							Mountpoint: "/",
						},
					},
				},
			},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			pt := tc.pt
			pt.relayout(tc.size)
			err := validatePTSize(pt)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, pt)
		})
	}
}
