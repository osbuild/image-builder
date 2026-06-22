package osbuild_test

import (
	"testing"

	"github.com/osbuild/image-builder/v73/pkg/osbuild"
	"github.com/stretchr/testify/assert"
)

func TestBootupdGenMetadataStage(t *testing.T) {
	expectedStage := &osbuild.Stage{
		Type: "org.osbuild.bootupd.gen-metadata",
	}
	stage := osbuild.NewBootupdGenMetadataStage()
	assert.Equal(t, stage, expectedStage)
}
