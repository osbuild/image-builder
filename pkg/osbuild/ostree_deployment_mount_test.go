package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/osbuild"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSTreeMountDeploymentDefaultSerialized(t *testing.T) {
	mntStage := osbuild.NewOSTreeDeploymentMountDefault("some-name", osbuild.OSTreeMountSourceMount)
	json, err := json.MarshalIndent(mntStage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "name": "some-name",
  "type": "org.osbuild.ostree.deployment",
  "options": {
    "source": "mount",
    "deployment": {
      "default": true
    }
  }
}`[1:])
}

func TestOSTreeMountDeploymentSerialized(t *testing.T) {
	mntStage := osbuild.NewOSTreeDeploymentMount("some-name", "some-osname", "some-ref", 0)
	json, err := json.MarshalIndent(mntStage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "name": "some-name",
  "type": "org.osbuild.ostree.deployment",
  "options": {
    "deployment": {
      "osname": "some-osname",
      "ref": "some-ref",
      "serial": 0
    }
  }
}`[1:])
}
