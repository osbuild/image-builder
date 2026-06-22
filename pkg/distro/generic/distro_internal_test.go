package generic

import (
	"testing"

	"github.com/osbuild/image-builder/v73/internal/common"
	"github.com/osbuild/image-builder/v73/pkg/arch"
	"github.com/osbuild/image-builder/v73/pkg/distro/defs"
	"github.com/stretchr/testify/assert"
)

func TestISOLabel(t *testing.T) {
	imgType := &imageType{
		arch: &architecture{
			arch: common.Must(arch.FromString("s390x")),
		},
	}
	d := &distribution{
		DistroYAML: defs.DistroYAML{
			Name:         "rhel-9.1",
			Product:      "some-product",
			ISOLabelTmpl: "name:{{.Distro.Name}},major:{{.Distro.MajorVersion}},minor:{{.Distro.MinorVersion}},product:{{.Product}},arch:{{.Arch}},iso-label:{{.ISOLabel}}",
		},
	}

	isoLabelFunc := d.getISOLabelFunc("iso-label")
	assert.Equal(t, "name:rhel,major:9,minor:1,product:some-product,arch:s390x,iso-label:iso-label", isoLabelFunc(imgType))
}
