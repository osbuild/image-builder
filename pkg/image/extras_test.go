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
