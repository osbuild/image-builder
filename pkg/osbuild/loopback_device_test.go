package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/osbuild"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoopbackDeviceOptionsSerializesAll(t *testing.T) {
	dev := osbuild.LoopbackDeviceOptions{
		Filename:   "foo.disk",
		Start:      12345,
		Size:       54321,
		SectorSize: common.ToPtr(uint64(4096)),
		Lock:       true,
		Partscan:   true,
	}
	json, err := json.MarshalIndent(dev, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "filename": "foo.disk",
  "start": 12345,
  "size": 54321,
  "sector-size": 4096,
  "lock": true,
  "partscan": true
}`[1:])
}

func TestLoopbackDeviceOptionsSerializesOmitEmptyHonored(t *testing.T) {
	dev := osbuild.LoopbackDeviceOptions{
		Filename: "foo.disk",
	}
	json, err := json.MarshalIndent(dev, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, string(json), `
{
  "filename": "foo.disk"
}`[1:])
}
