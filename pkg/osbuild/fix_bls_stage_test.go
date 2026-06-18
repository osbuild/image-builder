package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
)

func TestNewFixBLSStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.fix-bls",
		Options: &FixBLSStageOptions{},
	}
	actualStage := NewFixBLSStage(&FixBLSStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestFixBLSStageJSONRequireBootPrefix(t *testing.T) {
	opts := &FixBLSStageOptions{
		RequireBootPrefix: common.ToPtr(false),
	}
	stage := NewFixBLSStage(opts)
	data, err := json.MarshalIndent(stage, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, `{
  "type": "org.osbuild.fix-bls",
  "options": {
    "require_boot_prefix": false
  }
}`, string(data))
}

func TestFixBLSStageJSONPrefixAndRequireBootPrefix(t *testing.T) {
	opts := &FixBLSStageOptions{
		Prefix:            common.ToPtr(""),
		RequireBootPrefix: common.ToPtr(false),
	}
	stage := NewFixBLSStage(opts)
	data, err := json.MarshalIndent(stage, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, `{
  "type": "org.osbuild.fix-bls",
  "options": {
    "prefix": "",
    "require_boot_prefix": false
  }
}`, string(data))
}

func TestFixBLSStageJSONOmitEmpty(t *testing.T) {
	opts := &FixBLSStageOptions{}
	stage := NewFixBLSStage(opts)
	data, err := json.MarshalIndent(stage, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, `{
  "type": "org.osbuild.fix-bls",
  "options": {}
}`, string(data))
}
