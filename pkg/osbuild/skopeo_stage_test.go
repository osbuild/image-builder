package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestSkopeoStageJSONSmoke(t *testing.T) {
	inputs := osbuild.NewContainersInputForSources([]container.Spec{
		{
			ImageID:   "id-0",
			Source:    "registry.example.org/reg/img",
			LocalName: "local-name",
		},
	})

	stage := osbuild.NewSkopeoStageWithOCI("/some/path", inputs, nil)
	opts := stage.Options.(*osbuild.SkopeoStageOptions)
	opts.RemoveSignatures = common.ToPtr(true)

	stageJson, err := json.MarshalIndent(stage, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(stageJson), `{
  "type": "org.osbuild.skopeo",
  "inputs": {
    "images": {
      "type": "org.osbuild.containers",
      "origin": "org.osbuild.source",
      "references": {
        "id-0": {
          "name": "local-name"
        }
      }
    }
  },
  "options": {
    "destination": {
      "type": "oci",
      "path": "/some/path"
    },
    "remove-signatures": true
  }
}`)
}
