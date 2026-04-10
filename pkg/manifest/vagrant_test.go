package manifest_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/runner"
)

func TestNewVagrantMacAddress(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	// setup
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})

	// create a new random instance to use so we get the same "random" mac address each
	// time we run this test
	prng := rand.New(rand.NewSource(1))

	vagrantPipeline := manifest.NewVagrant(build, rawImage, osbuild.VagrantProviderVirtualBox, prng)

	assert.Equal(t, "08002752fdfc", vagrantPipeline.GetMacAddress())
}
