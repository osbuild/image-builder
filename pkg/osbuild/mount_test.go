package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/osbuild"
)

func TestNewMounts(t *testing.T) {
	assert := assert.New(t)

	{ // btrfs
		actual := osbuild.NewBtrfsMount("btrfs", "/dev/sda1", "/mnt/btrfs", "", "")
		expected := &osbuild.Mount{
			Name:    "btrfs",
			Type:    "org.osbuild.btrfs",
			Source:  "/dev/sda1",
			Target:  "/mnt/btrfs",
			Options: osbuild.BtrfsMountOptions{},
		}
		assert.Equal(expected, actual)
	}

	{ // ext4
		actual := osbuild.NewExt4Mount("ext4", "/dev/sda2", "/mnt/ext4")
		expected := &osbuild.Mount{
			Name:   "ext4",
			Type:   "org.osbuild.ext4",
			Source: "/dev/sda2",
			Target: "/mnt/ext4",
		}
		assert.Equal(expected, actual)
	}

	{ // fat
		actual := osbuild.NewFATMount("fat", "/dev/sda3", "/mnt/fat")
		expected := &osbuild.Mount{
			Name:   "fat",
			Type:   "org.osbuild.fat",
			Source: "/dev/sda3",
			Target: "/mnt/fat",
		}
		assert.Equal(expected, actual)
	}

	{ // xfs
		actual := osbuild.NewXfsMount("xfs", "/dev/sda4", "/mnt/xfs")
		expected := &osbuild.Mount{
			Name:   "xfs",
			Type:   "org.osbuild.xfs",
			Source: "/dev/sda4",
			Target: "/mnt/xfs",
		}
		assert.Equal(expected, actual)
	}

	{ // erofs
		actual := osbuild.NewErofsMount("erofs", "rootfs.img", "/mnt/erofs")
		expected := &osbuild.Mount{
			Name:   "erofs",
			Type:   "org.osbuild.erofs",
			Source: "rootfs.img",
			Target: "/mnt/erofs",
		}
		assert.Equal(expected, actual)
	}

	{ // squashfs
		actual := osbuild.NewSquashfsMount("squashfs", "rootfs.img", "/mnt/squashfs")
		expected := &osbuild.Mount{
			Name:   "squashfs",
			Type:   "org.osbuild.squashfs",
			Source: "rootfs.img",
			Target: "/mnt/squashfs",
		}
		assert.Equal(expected, actual)
	}
}

func TestMountJsonAll(t *testing.T) {
	mnt := &osbuild.Mount{
		Name:   "xfs",
		Type:   "org.osbuild.xfs",
		Source: "/dev/sda4",
		Target: "/mnt/xfs",
		//TODO: test "Options:" too
		Partition: common.ToPtr(1),
	}
	json, err := json.MarshalIndent(mnt, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "name": "xfs",
  "type": "org.osbuild.xfs",
  "source": "/dev/sda4",
  "target": "/mnt/xfs",
  "partition": 1
}`[1:])
}

func TestMountJsonOmitEmptyHonored(t *testing.T) {
	mnt := &osbuild.Mount{
		Name: "xfs",
		Type: "org.osbuild.xfs",
	}
	json, err := json.MarshalIndent(mnt, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "name": "xfs",
  "type": "org.osbuild.xfs"
}`[1:])
}
