package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestBindMountSerialized(t *testing.T) {
	mntStage := osbuild.NewBindMount("some-name", "mount://", "tree://")
	json, err := json.MarshalIndent(mntStage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "name": "some-name",
  "type": "org.osbuild.bind",
  "target": "tree://",
  "options": {
    "source": "mount://"
  }
}`[1:])
}

func TestBindMountErrorCheckingSrc(t *testing.T) {
	assert.PanicsWithError(t, `bind mount source must start with "mount://", got "invalid-src"`, func() {
		osbuild.NewBindMount("some-name", "invalid-src", "tree://")
	})
}

func TestBindMountErrorCheckingDst(t *testing.T) {
	assert.PanicsWithError(t, `bind mount target must start with "tree://", got "invalid-target"`, func() {
		osbuild.NewBindMount("some-name", "mount://", "invalid-target")
	})
}
