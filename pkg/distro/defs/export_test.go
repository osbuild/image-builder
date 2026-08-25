package defs

import (
	"math/rand"
	"os"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/distro"
)

type WhenCondition = whenCondition

func MockDataFS(path string) (restore func()) {
	saved := defaultDataFS
	defaultDataFS = os.DirFS(path)
	return func() {
		defaultDataFS = saved
	}
}

func LoaderForTest(path string) *Loader {
	return NewLoader(os.DirFS(path))
}

func ClearLoader(d *DistroYAML) {
	d.loader = nil
}

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
