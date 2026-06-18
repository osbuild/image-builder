package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestLVM2LVDeviceMarshal(t *testing.T) {
	opts := &osbuild.LVM2LVDeviceOptions{Volume: "some-volume"}
	dev := osbuild.NewLVM2LVDevice("some-parent", opts)
	b, err := json.MarshalIndent(dev, "", " ")
	assert.NoError(t, err)
	expectedJSON := `{
 "type": "org.osbuild.lvm2.lv",
 "parent": "some-parent",
 "options": {
  "volume": "some-volume"
 }
}`
	assert.Equal(t, expectedJSON, string(b))
}

func TestLVM2LVDeviceMarshalWithVGPartition(t *testing.T) {
	opts := &osbuild.LVM2LVDeviceOptions{
		Volume:    "some-volume",
		VGPartnum: common.ToPtr(4),
	}
	dev := osbuild.NewLVM2LVDevice("some-parent", opts)
	b, err := json.MarshalIndent(dev, "", " ")
	assert.NoError(t, err)
	expectedJSON := `{
 "type": "org.osbuild.lvm2.lv",
 "parent": "some-parent",
 "options": {
  "volume": "some-volume",
  "vg_partnum": 4
 }
}`
	assert.Equal(t, expectedJSON, string(b))
}
