package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
)

func TestNewGrub2InstStage(t *testing.T) {
	options := Grub2InstStageOptions{
		Filename: "img.raw",
		Platform: "i386-pc",
		Location: common.ToPtr(uint64(2048)),
		Core: CoreMkImage{
			Type:       "mkimage",
			PartLabel:  "gpt",
			Filesystem: "ext4",
		},
		Prefix: PrefixPartition{
			Type:      "partition",
			PartLabel: "gpt",
			Number:    common.ToPtr(uint(1)),
			Path:      "/boot/grub2",
		},
		SectorSize: common.ToPtr(uint64(512)),
	}

	expectedStage := &Stage{
		Type:    "org.osbuild.grub2.inst",
		Options: &options,
	}

	actualStage := NewGrub2InstStage(&options)
	assert.Equal(t, expectedStage, actualStage)
}

func TestMarshalGrub2InstStage(t *testing.T) {
	goodOptions := func() Grub2InstStageOptions {
		return Grub2InstStageOptions{
			Filename: "img.raw",
			Platform: "i386-pc",
			Location: common.ToPtr(uint64(2048)),
			Core: CoreMkImage{
				Type:       "mkimage",
				PartLabel:  "gpt",
				Filesystem: "ext4",
			},
			Prefix: PrefixPartition{
				Type:      "partition",
				PartLabel: "gpt",
				Number:    common.ToPtr(uint(1)),
				Path:      "/boot/grub2",
			},
			SectorSize: common.ToPtr(uint64(512)),
		}
	}

	{
		options := goodOptions()

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.NoError(t, err)
	}

	{
		options := goodOptions()
		options.Core.Type = "notmkimage"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Core.PartLabel = "notgpt"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Core.Filesystem = "apfs"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Prefix.Type = "notpartition"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Prefix.PartLabel = "notdos"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}
}

func TestMarshalGrub2InstStageISO9660(t *testing.T) {
	options := NewGrub2InstISO9660StageOption("image/eltorito.img", "/boot/grub2")
	stage := NewGrub2InstStage(options)
	_, err := json.Marshal(stage)
	assert.NoError(t, err)
}

func TestNewGrub2InstStageOptionSectorSize(t *testing.T) {
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
			// Create a minimal PartitionTable with a BIOS boot partition and /boot
			pt := &disk.PartitionTable{
				Type:       disk.PT_GPT,
				SectorSize: tc.ptSectorSize,
				Partitions: []disk.Partition{
					{
						Start:    1 * datasizes.MiB,
						Size:     datasizes.MiB,
						Type:     disk.BIOSBootPartitionGUID,
						Bootable: false,
					},
					{
						Start: 2 * datasizes.MiB,
						Size:  500 * datasizes.MiB,
						Payload: &disk.Filesystem{
							Type:       "ext4",
							Mountpoint: "/boot",
						},
					},
				},
			}

			options := NewGrub2InstStageOption("image.raw", pt, "i386-pc")

			if tc.expectedNil {
				assert.Nil(t, options.SectorSize)
			} else {
				assert.NotNil(t, options.SectorSize)
				assert.Equal(t, tc.expectedValue, *options.SectorSize)
			}
		})
	}
}
