package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

func TestNewGRUB2Stage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.grub2",
		Options: &GRUB2StageOptions{},
	}
	actualStage := NewGRUB2Stage(&GRUB2StageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func makePartitionTable(rootUUID string, boot *disk.Filesystem) *disk.PartitionTable {
	partitions := []disk.Partition{
		{
			Type: disk.FilesystemDataGUID,
			UUID: disk.RootPartitionUUID,
			Payload: &disk.Filesystem{
				Type:       "xfs",
				UUID:       rootUUID,
				Mountpoint: "/",
			},
		},
	}

	if boot != nil {
		partitions = append(partitions, disk.Partition{
			Type:    disk.FilesystemDataGUID,
			Payload: boot,
		})
	}

	return &disk.PartitionTable{
		Type:       disk.PT_GPT,
		Partitions: partitions,
	}
}

func TestNewGrub2StageOptions(t *testing.T) {
	rootUUID := "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75"

	t.Run("ext4 boot partition", func(t *testing.T) {
		bootUUID := "dbd21911-1c4e-4107-8a9f-14fe6e751358"
		pt := makePartitionTable(rootUUID, &disk.Filesystem{
			Type:       "ext4",
			UUID:       bootUUID,
			Mountpoint: "/boot",
		})

		opts := NewGrub2StageOptions(pt, "root=/dev/sda2", "5.14.0", true, "", "fedora", true)

		require.NotNil(t, opts.BootFilesystemUUID)
		assert.Equal(t, bootUUID, *opts.BootFilesystemUUID)
	})

	t.Run("vfat boot partition", func(t *testing.T) {
		vfatSerial := "7B77-95E7"
		pt := makePartitionTable(rootUUID, &disk.Filesystem{
			Type:       "vfat",
			UUID:       vfatSerial,
			Mountpoint: "/boot",
		})

		opts := NewGrub2StageOptions(pt, "root=/dev/sda2", "5.14.0", true, "", "fedora", true)

		require.NotNil(t, opts.BootFilesystemUUID)
		assert.Equal(t, vfatSerial, *opts.BootFilesystemUUID)
	})

	t.Run("no boot partition", func(t *testing.T) {
		pt := makePartitionTable(rootUUID, nil)

		opts := NewGrub2StageOptions(pt, "root=/dev/sda2", "5.14.0", true, "", "fedora", true)

		assert.Nil(t, opts.BootFilesystemUUID)
	})

	t.Run("ext4 boot with invalid UUID panics", func(t *testing.T) {
		pt := makePartitionTable(rootUUID, &disk.Filesystem{
			Type:       "ext4",
			UUID:       "not-a-uuid",
			Mountpoint: "/boot",
		})

		assert.Panics(t, func() {
			NewGrub2StageOptions(pt, "root=/dev/sda2", "5.14.0", true, "", "fedora", true)
		})
	})

	t.Run("common fields are set correctly", func(t *testing.T) {
		pt := makePartitionTable(rootUUID, nil)

		opts := NewGrub2StageOptions(pt, "console=ttyS0", "5.14.0", true, "i386-pc", "fedora", true)

		assert.Equal(t, 2, opts.CompatVersion)
		assert.Equal(t, rootUUID, opts.RootFilesystemUUID.String())
		assert.Equal(t, "console=ttyS0", opts.KernelOptions)
		assert.Equal(t, "i386-pc", opts.Legacy)
		assert.Equal(t, common.ToPtr(false), opts.WriteCmdLine)
		assert.Equal(t, "ffffffffffffffffffffffffffffffff-5.14.0", opts.SavedEntry)
		require.NotNil(t, opts.Config)
		assert.Equal(t, "saved", opts.Config.Default)
		require.NotNil(t, opts.UEFI)
		assert.Equal(t, "fedora", opts.UEFI.Vendor)
		assert.True(t, opts.UEFI.Install)
		assert.True(t, opts.UEFI.Unified)
	})
}
