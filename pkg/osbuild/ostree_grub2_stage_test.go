package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSTreeGrub2StageJsonMinimal(t *testing.T) {
	expectedJson := `{
  "type": "org.osbuild.ostree.grub2",
  "inputs": {
    "tree": {
      "type": "org.osbuild.tree",
      "origin": "org.osbuild.pipeline",
      "references": [
        "name:input-pipeline"
      ]
    }
  },
  "options": {
    "filename": "grub.cfg"
  }
}`

	opts := &osbuild.OSTreeGrub2StageOptions{
		Filename: "grub.cfg",
	}
	stage := osbuild.NewOSTreeGrub2Stage(opts, "input-pipeline")
	require.NotNil(t, stage)

	js, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, expectedJson, string(js))
}

func TestOSTreeGrub2StageJsonMounts(t *testing.T) {
	expectedJson := `{
  "type": "org.osbuild.ostree.grub2",
  "options": {
    "filename": "grub.cfg",
    "source": "mount://-/"
  },
  "devices": {
    "disk": {
      "type": "org.osbuild.loopback",
      "options": {
        "filename": "disk.img",
        "partscan": true
      }
    }
  },
  "mounts": [
    {
      "name": "-",
      "type": "org.osbuild.xfs",
      "source": "disk",
      "target": "/",
      "partition": 4
    }
  ]
}`

	opts := &osbuild.OSTreeGrub2StageOptions{
		Filename: "grub.cfg",
		Source:   "mount://-/",
	}
	devices := make(map[string]osbuild.Device)
	devices["disk"] = osbuild.Device{
		Type: "org.osbuild.loopback",
		Options: osbuild.LoopbackDeviceOptions{
			Filename: "disk.img",
			Partscan: true,
		},
	}
	mounts := []osbuild.Mount{
		{
			Name:      "-",
			Type:      "org.osbuild.xfs",
			Source:    "disk",
			Target:    "/",
			Partition: common.ToPtr(4),
		},
	}

	stage := osbuild.NewOSTreeGrub2MountsStage(opts, nil, devices, mounts)
	require.NotNil(t, stage)

	js, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, expectedJson, string(js))
}
