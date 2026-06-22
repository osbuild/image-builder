package dependencies_test

import (
	"testing"

	"github.com/osbuild/image-builder/v73/data/dependencies"
	"github.com/stretchr/testify/assert"
)

func TestMinimumOSBuildVersion(t *testing.T) {
	assert.Equal(t, "183", dependencies.MinimumOSBuildVersion())
}
