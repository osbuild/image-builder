package dependencies_test

import (
	"testing"

	"github.com/osbuild/image-builder/data/dependencies"
	"github.com/stretchr/testify/assert"
)

func TestMinimumOSBuildVersion(t *testing.T) {
	assert.Equal(t, "185", dependencies.MinimumOSBuildVersion())
}
