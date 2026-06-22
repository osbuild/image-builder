package disk_test

import (
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/disk"
)

func TestImplementsInterfacesCompileTimeCheckRaw(t *testing.T) {
	var _ = disk.PayloadEntity(&disk.Raw{})
}
