package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/v73/pkg/container"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func TestGenContainerStorageStagesTrivial(t *testing.T) {
	storagePath := ""
	var containerSpecs []container.Spec
	stages := osbuild.GenContainerStorageStages(storagePath, containerSpecs)
	assert.Equal(t, len(stages), 0)
}

func TestGenContainerStorageStagesSkopeoOnly(t *testing.T) {
	storagePath := ""
	containerSpecs := []container.Spec{
		{
			LocalName: "some-name",
			ImageID:   "sha256:1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
		},
	}
	stages := osbuild.GenContainerStorageStages(storagePath, containerSpecs)
	assert.Equal(t, len(stages), 1)
	assert.Equal(t, stages[0].Type, "org.osbuild.skopeo")
	assert.Equal(t, stages[0].Inputs.(osbuild.SkopeoStageInputs).Images.Type, "org.osbuild.containers")
}

func TestGenContainerStorageStagesSkopeoMixed(t *testing.T) {
	storagePath := ""
	containerSpecs := []container.Spec{
		{
			LocalName: "some-name",
			ImageID:   "sha256:1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
		},
		{
			LocalName:    "other-name",
			ImageID:      "sha256:aabbccf64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
			LocalStorage: true,
		},
	}
	stages := osbuild.GenContainerStorageStages(storagePath, containerSpecs)
	assert.Equal(t, len(stages), 2)
	assert.Equal(t, stages[0].Type, "org.osbuild.skopeo")
	assert.Equal(t, stages[0].Inputs.(osbuild.SkopeoStageInputs).Images.Type, "org.osbuild.containers")
	assert.Equal(t, stages[1].Type, "org.osbuild.skopeo")
	assert.Equal(t, stages[1].Inputs.(osbuild.SkopeoStageInputs).Images.Type, "org.osbuild.containers-storage")
}

func TestGenContainerStorageStagesSkopeoWithStoragePath(t *testing.T) {
	storagePath := "/some/storage/path"
	containerSpecs := []container.Spec{
		{
			LocalName: "some-name",
			ImageID:   "sha256:1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
		},
		{
			LocalName:    "other-name",
			ImageID:      "sha256:aabbccf64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
			LocalStorage: true,
		},
	}
	stages := osbuild.GenContainerStorageStages(storagePath, containerSpecs)
	assert.Equal(t, len(stages), 3)
	assert.Equal(t, stages[0].Type, "org.osbuild.containers.storage.conf")
	assert.Equal(t, stages[1].Type, "org.osbuild.skopeo")
	assert.Equal(t, stages[1].Inputs.(osbuild.SkopeoStageInputs).Images.Type, "org.osbuild.containers")
	assert.Equal(t, stages[2].Type, "org.osbuild.skopeo")
	assert.Equal(t, stages[2].Inputs.(osbuild.SkopeoStageInputs).Images.Type, "org.osbuild.containers-storage")
}

func TestGenContainerStorageStagesIntegration(t *testing.T) {
	storagePath := "/some/storage/path.conf"
	containerSpecs := []container.Spec{
		{
			LocalName: "some-name",
			ImageID:   "sha256:1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
		},
		{
			LocalName:    "local-name",
			ImageID:      "sha256:aabbccf64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
			LocalStorage: true,
		},
	}
	stages := osbuild.GenContainerStorageStages(storagePath, containerSpecs)
	jsonOutput, err := json.MarshalIndent(stages, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, string(jsonOutput), `[
  {
    "type": "org.osbuild.containers.storage.conf",
    "options": {
      "filename": "/etc/containers/storage.conf",
      "config": {
        "storage": {
          "options": {
            "additionalimagestores": [
              "/some/storage/path.conf"
            ]
          }
        }
      }
    }
  },
  {
    "type": "org.osbuild.skopeo",
    "inputs": {
      "images": {
        "type": "org.osbuild.containers",
        "origin": "org.osbuild.source",
        "references": {
          "sha256:1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db": {
            "name": "some-name"
          }
        }
      }
    },
    "options": {
      "destination": {
        "type": "containers-storage",
        "storage-path": "/some/storage/path.conf"
      }
    }
  },
  {
    "type": "org.osbuild.skopeo",
    "inputs": {
      "images": {
        "type": "org.osbuild.containers-storage",
        "origin": "org.osbuild.source",
        "references": {
          "sha256:aabbccf64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db": {
            "name": "local-name"
          }
        }
      }
    },
    "options": {
      "destination": {
        "type": "containers-storage",
        "storage-path": "/some/storage/path.conf"
      }
    }
  }
]`)
}
