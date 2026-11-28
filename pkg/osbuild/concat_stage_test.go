package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestNewConcatStage(t *testing.T) {
	stage := osbuild.NewConcatStage(&osbuild.ConcatStageOptions{
		Filename:  "initrd.img",
		Files:     []string{"input://microcode/image.cpio", "input://main/image.cpio"},
		Alignment: 4,
	}, osbuild.ConcatStageInputs{
		"main":      osbuild.NewTreeInput("name:userspace"),
		"microcode": osbuild.NewFilesInput(osbuild.NewFilesInputPipelineObjectRef("early", "image.cpio", nil)),
	})

	data, err := json.Marshal(stage)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "org.osbuild.concat",
		"inputs": {
			"main": {
				"type": "org.osbuild.tree",
				"origin": "org.osbuild.pipeline",
				"references": ["name:userspace"]
			},
			"microcode": {
				"type": "org.osbuild.files",
				"origin": "org.osbuild.pipeline",
				"references": {"name:early": {"file": "image.cpio"}}
			}
		},
		"options": {
			"filename": "initrd.img",
			"files": ["input://microcode/image.cpio", "input://main/image.cpio"],
			"alignment": 4
		}
	}`, string(data))
}

func TestConcatStageDefaultAlignment(t *testing.T) {
	data, err := json.Marshal(osbuild.ConcatStageOptions{
		Filename: "archive",
		Files:    []string{"input://first/part", "input://second/part"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"filename": "archive",
		"files": ["input://first/part", "input://second/part"]
	}`, string(data))
}
