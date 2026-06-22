package osbuild

import (
	"math/rand"
	"testing"

	"github.com/osbuild/image-builder/v73/internal/testdisk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFSTabStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.fstab",
		Options: &FSTabStageOptions{},
	}
	actualStage := NewFSTabStage(&FSTabStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestAddFilesystem(t *testing.T) {
	options := &FSTabStageOptions{}
	filesystems := []*FSTabEntry{
		{
			UUID:    "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
			VFSType: "ext4",
			Path:    "/",
			Options: "defaults",
			Freq:    1,
			PassNo:  1,
		},
		{
			UUID:    "bba22bf4-f153-4541-b6c7-0332c0dfaeac",
			VFSType: "xfs",
			Path:    "/home",
			Options: "defaults",
			Freq:    1,
			PassNo:  2,
		},
		{
			UUID:    "cca22bf4-f153-4541-b6c7-0332c0dfaeac",
			VFSType: "xfs",
			Path:    "/var",
			Options: "defaults",
			Freq:    1,
			PassNo:  1,
		},
	}

	for i, fs := range filesystems {
		options.AddFilesystem(fs.UUID, fs.VFSType, fs.Path, fs.Options, fs.Freq, fs.PassNo)
		assert.Equal(t, options.FileSystems[i], fs)
	}
	assert.Equal(t, len(filesystems), len(options.FileSystems))
}

func TestNewFSTabStageOptions(t *testing.T) {
	expectedOptions := map[string]FSTabStageOptions{
		// The names must match the ones in testdisk.TestPartitionTables
		"plain": {
			FileSystems: []*FSTabEntry{
				{UUID: "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", VFSType: "xfs", Path: "/", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8", VFSType: "xfs", Path: "/boot", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "7B77-95E7", VFSType: "vfat", Path: "/boot/efi", Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt", Freq: 0, PassNo: 2},
			},
		},
		"plain-swap": {
			FileSystems: []*FSTabEntry{
				{UUID: "fb180daf-48a7-4ee0-b10d-394651850fd4", VFSType: "xfs", Path: "/", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8", VFSType: "xfs", Path: "/boot", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", VFSType: "swap", Path: "none", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "7B77-95E7", VFSType: "vfat", Path: "/boot/efi", Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt", Freq: 0, PassNo: 2},
			},
		},
		"plain-noboot": {
			FileSystems: []*FSTabEntry{
				{UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8", VFSType: "xfs", Path: "/", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "7B77-95E7", VFSType: "vfat", Path: "/boot/efi", Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt", Freq: 0, PassNo: 2},
			},
		},
		"luks": {
			FileSystems: []*FSTabEntry{
				{UUID: "fb180daf-48a7-4ee0-b10d-394651850fd4", VFSType: "xfs", Path: "/", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8", VFSType: "xfs", Path: "/boot", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "7B77-95E7", VFSType: "vfat", Path: "/boot/efi", Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt", Freq: 0, PassNo: 2},
			},
		},
		"luks+lvm": {
			FileSystems: []*FSTabEntry{
				{UUID: "fb180daf-48a7-4ee0-b10d-394651850fd4", VFSType: "xfs", Path: "/", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8", VFSType: "xfs", Path: "/boot", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "a178892e-e285-4ce1-9114-55780875d64e", VFSType: "xfs", Path: "/home", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "7B77-95E7", VFSType: "vfat", Path: "/boot/efi", Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt", Freq: 0, PassNo: 2},
			},
		},
		"btrfs": {
			FileSystems: []*FSTabEntry{
				{UUID: "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", VFSType: "btrfs", Path: "/", Options: "subvol=root", Freq: 0, PassNo: 0},
				{UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8", VFSType: "xfs", Path: "/boot", Options: "defaults", Freq: 0, PassNo: 0},
				{UUID: "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", VFSType: "btrfs", Path: "/var", Options: "subvol=var", Freq: 0, PassNo: 0},
				{UUID: "7B77-95E7", VFSType: "vfat", Path: "/boot/efi", Options: "defaults,uid=0,gid=0,umask=077,shortname=winnt", Freq: 0, PassNo: 2},
			},
		},
	}
	// Use the test partition tables from the disk package.
	for name, pt := range testdisk.TestPartitionTables() {
		// use a different name for the internal testing argument so we can
		// refer to the global test by t.Name() in the error message
		t.Run(name, func(ts *testing.T) {
			require := require.New(ts)

			// math/rand is good enough in this case
			/* #nosec G404 */
			rng := rand.New(rand.NewSource(0))
			// populate UUIDs
			pt.GenerateUUIDs(rng)

			// print an informative failure message if a new test partition
			// table is added and this test is not updated (instead of failing
			// at the final Equal() check)
			exp, ok := expectedOptions[name]
			require.True(ok, "expected test result not defined for test partition table %q: please update the %s test", name, t.Name())

			options, err := NewFSTabStageOptions(&pt)
			require.NoError(err)
			require.NotNil(options)
			require.Equal(exp, *options)
		})
	}
}
