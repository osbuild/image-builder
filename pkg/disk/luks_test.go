package disk_test

import (
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/disk"
)

func TestImplementsInterfacesCompileTimeCheckLUKS(t *testing.T) {
	var _ = disk.Container(&disk.LUKSContainer{})
}
