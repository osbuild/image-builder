package generic

import (
	"math/rand"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
)

// math/rand is good enough in this case
/* #nosec G404 */
var rng = rand.New(rand.NewSource(0))

func GetPartitionTable(it distro.ImageType) (*disk.PartitionTable, error) {
	return it.(*imageType).getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
}

type (
	ImageType    = imageType
	Distribution = distribution
)

func (t *imageType) GetDefaultImageConfig() *distro.ImageConfig {
	return t.getDefaultImageConfig()
}

func ImageTypeCheckOptions(it *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	return it.checkOptions(bp, options)
}
