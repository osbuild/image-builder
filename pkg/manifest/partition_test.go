package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
)

func TestFindPartitionByMountpoint(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Start: 1048576,
				Size:  524288000,
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: "/boot",
				},
			},
			{
				Start: 525336576,
				Size:  5368709120,
				Payload: &disk.Filesystem{
					Type:       "xfs",
					Mountpoint: "/",
				},
			},
		},
	}

	tests := []struct {
		name       string
		mountpoint string
		wantStart  uint64
		wantSize   uint64
		wantErr    string
	}{
		{
			name:       "find boot",
			mountpoint: "/boot",
			wantStart:  1048576,
			wantSize:   524288000,
		},
		{
			name:       "find root",
			mountpoint: "/",
			wantStart:  525336576,
			wantSize:   5368709120,
		},
		{
			name:       "not found",
			mountpoint: "/var",
			wantErr:    `partition with mountpoint "/var" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part, err := manifest.FindPartitionByMountpoint(pt, tt.mountpoint)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, part.Start)
			assert.Equal(t, uint64(tt.wantSize), uint64(part.Size))
		})
	}
}

func TestFindPartitionByMountpointLVM(t *testing.T) {
	pt := &disk.PartitionTable{
		Type: disk.PT_GPT,
		Partitions: []disk.Partition{
			{
				Start: 1048576,
				Size:  524288000,
				Payload: &disk.Filesystem{
					Type:       "ext4",
					Mountpoint: "/boot",
				},
			},
			{
				Start: 525336576,
				Size:  10737418240,
				Payload: &disk.LVMVolumeGroup{
					Name: "rootvg",
					LogicalVolumes: []disk.LVMLogicalVolume{
						{
							Name: "rootlv",
							Size: 5368709120,
							Payload: &disk.Filesystem{
								Type:       "xfs",
								Mountpoint: "/",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		mountpoint string
		wantStart  uint64
		wantSize   uint64
		wantErr    string
	}{
		{
			name:       "plain partition still works",
			mountpoint: "/boot",
			wantStart:  1048576,
			wantSize:   524288000,
		},
		{
			name:       "lvm mountpoint rejected",
			mountpoint: "/",
			wantErr:    `mountpoint "/" is not on a plain partition`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part, err := manifest.FindPartitionByMountpoint(pt, tt.mountpoint)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, part.Start)
			assert.Equal(t, uint64(tt.wantSize), uint64(part.Size))
		})
	}
}
