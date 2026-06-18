package disk_test

import (
	"testing"

	"github.com/osbuild/image-builder/pkg/disk"
)

func TestImplementsInterfacesCompileTimeCheckRaw(t *testing.T) {
	var _ = disk.PayloadEntity(&disk.Raw{})
}
