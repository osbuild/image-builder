package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMkS390ImageStage(t *testing.T) {
	expectedJSON := `{
  "type": "org.osbuild.mks390image",
  "options": {
    "kernel": "images/kernel.img",
    "initrd": "images/initrd.img",
    "config": "images/cdboot.prm",
    "image": "images/cdboot.img"
  }
}`

	options := MkS390ImageStageOptions{
		Kernel: "images/kernel.img",
		Initrd: "images/initrd.img",
		Config: "images/cdboot.prm",
		Image:  "images/cdboot.img",
	}
	stage := NewMkS390ImageStage(&options)
	b, err := json.MarshalIndent(stage, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(b))
}
