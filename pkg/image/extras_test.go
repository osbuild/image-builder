package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/image"
)

func TestSysextPipelineName(t *testing.T) {
	assert.Equal(t, "sysext-nginx-erofs", image.SysextPipelineName("nginx", "erofs"))
}
