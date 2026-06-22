package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func TestNewBootctlInstallRootStage(t *testing.T) {
	opts := &osbuild.BootctlInstallRootStageOptions{
		Root: "mount://-/",
	}

	stage := osbuild.NewBootctlInstallRootStage(opts, nil, nil)
	assert.Equal(t, "org.osbuild.bootctl.install.root", stage.Type)
	assert.Equal(t, opts, stage.Options)
}

func TestBootctlInstallRootStageJSON(t *testing.T) {
	opts := &osbuild.BootctlInstallRootStageOptions{
		Root:     "mount://-/",
		ESPPath:  "/boot/efi",
		BootPath: "/boot",
	}

	stage := osbuild.NewBootctlInstallRootStage(opts, nil, nil)
	data, err := json.MarshalIndent(stage, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, `{
  "type": "org.osbuild.bootctl.install.root",
  "options": {
    "root": "mount://-/",
    "esp-path": "/boot/efi",
    "boot-path": "/boot"
  }
}`, string(data))
}

func TestBootctlInstallRootStageJSONAllOptions(t *testing.T) {
	opts := &osbuild.BootctlInstallRootStageOptions{
		Root:               "mount://-/",
		ESPPath:            "/boot/efi",
		BootPath:           "/boot",
		RelaxESPChecks:     true,
		RandomSeed:         "no",
		MakeEntryDirectory: "yes",
		EntryToken:         "os-id",
	}

	stage := osbuild.NewBootctlInstallRootStage(opts, nil, nil)
	data, err := json.MarshalIndent(stage, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, `{
  "type": "org.osbuild.bootctl.install.root",
  "options": {
    "root": "mount://-/",
    "esp-path": "/boot/efi",
    "boot-path": "/boot",
    "relax-esp-checks": true,
    "random-seed": "no",
    "make-entry-directory": "yes",
    "entry-token": "os-id"
  }
}`, string(data))
}

func TestBootctlInstallRootStageJSONOmitEmpty(t *testing.T) {
	opts := &osbuild.BootctlInstallRootStageOptions{
		Root: "mount://-/",
	}

	stage := osbuild.NewBootctlInstallRootStage(opts, nil, nil)
	data, err := json.MarshalIndent(stage, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, `{
  "type": "org.osbuild.bootctl.install.root",
  "options": {
    "root": "mount://-/"
  }
}`, string(data))
}
