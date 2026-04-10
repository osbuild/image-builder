package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/osbuild"
)

func TestSquashfsStageJsonMinimal(t *testing.T) {
	expectedJson := `{
  "type": "org.osbuild.squashfs",
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
    "filename": "disk.img",
    "compression": {
      "method": "xz"
    }
  }
}`

	opts := &osbuild.SquashfsStageOptions{
		Filename: "disk.img",
		Compression: osbuild.FSCompression{
			Method: "xz",
		},
	}
	stage := osbuild.NewSquashfsStage(opts, "input-pipeline")
	require.NotNil(t, stage)

	js, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, expectedJson, string(js))
}

func TestSquashfsStageJsonFull(t *testing.T) {
	expectedJson := `{
  "type": "org.osbuild.squashfs",
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
    "filename": "disk.img",
    "source": "mount://-/",
    "exclude_paths": [
      "boot/efi/.*",
      "boot/initramfs-.*"
    ],
    "compression": {
      "method": "xz",
      "options": {
        "bcj": "x86"
      }
    }
  }
}`

	opts := &osbuild.SquashfsStageOptions{
		Filename: "disk.img",
		Compression: osbuild.FSCompression{
			Method: "xz",
			Options: &osbuild.FSCompressionOptions{
				BCJ: "x86",
			},
		},
		ExcludePaths: []string{
			"boot/efi/.*",
			"boot/initramfs-.*",
		},
		Source: "mount://-/",
	}
	stage := osbuild.NewSquashfsStage(opts, "input-pipeline")
	require.NotNil(t, stage)

	js, err := json.MarshalIndent(stage, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, expectedJson, string(js))
}
