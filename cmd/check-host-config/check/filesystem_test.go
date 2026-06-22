package check_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/image-builder/v73/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesystemCheck(t *testing.T) {
	lsblkCmd := "lsblk -J -o MOUNTPOINTS,FSTYPE"

	lsblkWithAll := []byte(`{"blockdevices": [
		{"mountpoints": ["/sys"], "fstype": "sysfs"},
		{"mountpoints": ["/"], "fstype": "xfs"},
		{"mountpoints": ["/var"], "fstype": "ext4"},
		{"mountpoints": ["/home"], "fstype": "ext4"},
		{"mountpoints": ["/data"], "fstype": "ext4"},
		{"mountpoints": ["/opt"], "fstype": "ext4"},
		{"mountpoints": ["/srv"], "fstype": "xfs"},
		{"mountpoints": ["/home/shadowman"], "fstype": "ext4"},
		{"mountpoints": ["/foo"], "fstype": "ext4"},
		{"mountpoints": ["/media"], "fstype": "ext4"},
		{"mountpoints": ["/root"], "fstype": "ext4"},
		{"mountpoints": ["/usr"], "fstype": "ext4"},
		{"mountpoints": ["[SWAP]"], "fstype": "swap"}
	]}`)

	lsblkNoSwap := []byte(`{"blockdevices": [
		{"mountpoints": ["/"], "fstype": "xfs"},
		{"mountpoints": ["/data"], "fstype": "ext4"}
	]}`)

	tests := []struct {
		name       string
		customFS   []blueprint.FilesystemCustomization
		customDisk *blueprint.DiskCustomization
		mockExec   map[string]ExecResult
		wantErr    error
	}{
		{
			name:    "skip when no filesystem customizations",
			wantErr: check.ErrCheckSkipped,
		},
		{
			name: "pass with filesystem customizations",
			customFS: []blueprint.FilesystemCustomization{
				{Mountpoint: "/data", MinSize: 1073741824},
				{Mountpoint: "/home", MinSize: 2147483648},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stdout: lsblkWithAll},
			},
		},
		{
			name: "fail with missing filesystem mountpoint",
			customFS: []blueprint.FilesystemCustomization{
				{Mountpoint: "/data", MinSize: 1073741824},
				{Mountpoint: "/nonexistent", MinSize: 1073741824},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stdout: lsblkWithAll},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "pass with disk plain partitions",
			customDisk: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "plain",
						MinSize: 1073741824,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/data",
							FSType:     "ext4",
						},
					},
					{
						Type:    "plain",
						MinSize: 1073741824,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							FSType: "swap",
						},
					},
				},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stdout: lsblkWithAll},
			},
		},
		{
			name: "pass with disk lvm partitions",
			customDisk: &blueprint.DiskCustomization{
				Type: "gpt",
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "lvm",
						MinSize: 10737418240,
						VGCustomization: blueprint.VGCustomization{
							Name: "testvg",
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name:    "homelv",
									MinSize: 2147483648,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										Mountpoint: "/home",
										FSType:     "ext4",
									},
								},
								{
									Name:    "swap-lv",
									MinSize: 1073741824,
									FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
										FSType: "swap",
									},
								},
							},
						},
					},
				},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stdout: lsblkWithAll},
			},
		},
		{
			name: "pass with disk btrfs partitions",
			customDisk: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "btrfs",
						MinSize: 10737418240,
						BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{Name: "subvol-home", Mountpoint: "/home"},
								{Name: "subvol-opt", Mountpoint: "/opt"},
							},
						},
					},
				},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stdout: lsblkWithAll},
			},
		},
		{
			name: "fail when swap expected but not active",
			customDisk: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type:    "plain",
						MinSize: 1073741824,
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							FSType: "swap",
						},
					},
				},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stdout: lsblkNoSwap},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "fail when lsblk fails",
			customFS: []blueprint.FilesystemCustomization{
				{Mountpoint: "/data", MinSize: 1073741824},
			},
			mockExec: map[string]ExecResult{
				lsblkCmd: {Stderr: []byte("command not found"), Code: 127, Err: fmt.Errorf("exit status 127")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("filesystem")
			require.True(t, found, "filesystem check not found")

			config := buildConfig(&blueprint.Customizations{
				Filesystem: tt.customFS,
				Disk:       tt.customDisk,
			})

			err := chk.Func(chk.Meta, config)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
