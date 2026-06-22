package disk_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/disk"
)

func TestImplementsInterfacesCompileTimeCheckFilesystem(t *testing.T) {
	var _ = disk.Mountable(&disk.Filesystem{})
	var _ = disk.UniqueEntity(&disk.Filesystem{})
	var _ = disk.FSTabEntity(&disk.Filesystem{})
}

func TestMkfsOptionsClone(t *testing.T) {
	Geometry := disk.MkfsOptionGeometry{
		Heads:           12,
		SectorsPerTrack: 21,
	}
	orig := disk.MkfsOptions{
		Verity:   true,
		Geometry: &Geometry,
	}
	clone := orig.Clone()
	assert.Equal(t, orig, clone)
	assert.False(t, reflect.ValueOf(orig.Geometry).Pointer() == reflect.ValueOf(clone.Geometry).Pointer())

}
