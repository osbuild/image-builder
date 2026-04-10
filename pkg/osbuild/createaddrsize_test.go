package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreateaddrsizeStage(t *testing.T) {
	expectedJSON := `{
  "type": "org.osbuild.createaddrsize",
  "options": {
    "initrd": "images/initrd.img",
    "addrsize": "images/initrd.addrsize"
  }
}`

	options := CreateaddrsizeStageOptions{
		Initrd:   "images/initrd.img",
		Addrsize: "images/initrd.addrsize",
	}
	stage := NewCreateaddrsizeStage(&options)
	b, err := json.MarshalIndent(stage, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, string(b))
}
