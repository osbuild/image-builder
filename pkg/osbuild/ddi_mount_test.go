package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestDDIMountBasic(t *testing.T) {
	mnt := osbuild.NewDDIMount("ddi", "image.raw", "/sysroot")
	data, err := json.MarshalIndent(mnt, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, `
{
  "name": "ddi",
  "type": "org.osbuild.ddi",
  "source": "image.raw",
  "target": "/sysroot"
}`[1:], string(data))
}

func TestDDIMountAllOptions(t *testing.T) {
	mnt := osbuild.NewDDIMount("ddi", "image.raw", "/sysroot")
	mnt.Options = &osbuild.DDIMountOptions{
		GrowFS:      common.ToPtr(false),
		ReadOnly:    common.ToPtr(true),
		Fsck:        common.ToPtr(false),
		Discard:     "all",
		ImagePolicy: "root=verity",
		ImageFilter: "usr",
	}
	data, err := json.MarshalIndent(mnt, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, `
{
  "name": "ddi",
  "type": "org.osbuild.ddi",
  "source": "image.raw",
  "target": "/sysroot",
  "options": {
    "growfs": false,
    "read-only": true,
    "fsck": false,
    "discard": "all",
    "image-policy": "root=verity",
    "image-filter": "usr"
  }
}`[1:], string(data))
}

func TestDDIMountPartialOptions(t *testing.T) {
	mnt := osbuild.NewDDIMount("ddi", "image.raw", "/sysroot")
	mnt.Options = &osbuild.DDIMountOptions{
		ReadOnly: common.ToPtr(true),
	}
	data, err := json.MarshalIndent(mnt, "", "  ")
	require.Nil(t, err)
	assert.Equal(t, `
{
  "name": "ddi",
  "type": "org.osbuild.ddi",
  "source": "image.raw",
  "target": "/sysroot",
  "options": {
    "read-only": true
  }
}`[1:], string(data))
}
