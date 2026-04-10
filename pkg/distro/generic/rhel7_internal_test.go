package generic

import (
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
)

func TestRhel7_ESP(t *testing.T) {
	var distros []distro.Distro
	for _, distroName := range []string{"rhel-7.9"} {
		distros = append(distros, common.Must(New(distroName)))
	}

	distro_test_common.TestESP(t, distros, func(i distro.ImageType) (*disk.PartitionTable, error) {
		it := i.(*imageType)
		return it.getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
	})
}
