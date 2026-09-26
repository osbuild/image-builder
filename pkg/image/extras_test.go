package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/image"
)

func TestSysextPipelineName(t *testing.T) {
	assert.Equal(t, "sysext-nginx-erofs", image.SysextPipelineName("nginx", "erofs"))
}

func TestPartitionPipelineName(t *testing.T) {
	assert.Equal(t, "partition-rootfs", image.PartitionPipelineName("rootfs", ""))
	assert.Equal(t, "partition-boot-xz", image.PartitionPipelineName("boot", "xz"))
}

func TestFilePrepPipelineName(t *testing.T) {
	assert.Equal(t, "file-kernel-prep", image.FilePrepPipelineName("kernel"))
	assert.Equal(t, "file-initrd-prep", image.FilePrepPipelineName("initrd"))
}

func TestFilePipelineName(t *testing.T) {
	assert.Equal(t, "file-kernel", image.FilePipelineName("kernel", ""))
	assert.Equal(t, "file-initrd-xz", image.FilePipelineName("initrd", "xz"))
}
