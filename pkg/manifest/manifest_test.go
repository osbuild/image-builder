package manifest_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/manifest"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func TestDistroUnmarshal(t *testing.T) {
	var distro manifest.Distro

	for _, tc := range []struct {
		inp      string
		expected manifest.Distro
	}{
		{`"rhel-10"`, manifest.DISTRO_EL10},
		{`"rhel-9"`, manifest.DISTRO_EL9},
		{`"rhel-8"`, manifest.DISTRO_EL8},
		{`"rhel-7"`, manifest.DISTRO_EL7},
		{`"fedora"`, manifest.DISTRO_FEDORA},
	} {
		err := distro.UnmarshalJSON([]byte(tc.inp))
		assert.NoError(t, err)
		assert.Equal(t, tc.expected, distro)
		// and distro can be converted back to the same string
		assert.Equal(t, tc.inp, fmt.Sprintf("%q", distro))
	}
}

func TestUnknownDistroPanic(t *testing.T) {
	assert.PanicsWithError(t, "unknown distro: 911", func() {
		_ = manifest.Distro(911).String()
	})
}

func TestAllDistrosHaveNames(t *testing.T) {
	assert.Equal(t, len(manifest.DistroNames), int(manifest.DISTRO_COUNT))
}

func findStage(name string, stages []*osbuild.Stage) *osbuild.Stage {
	for _, s := range stages {
		if s.Type == name {
			return s
		}
	}
	return nil
}

func findStages(name string, stages []*osbuild.Stage) []*osbuild.Stage {
	var foundStages []*osbuild.Stage
	for _, s := range stages {
		if s.Type == name {
			foundStages = append(foundStages, s)
		}
	}
	return foundStages
}
