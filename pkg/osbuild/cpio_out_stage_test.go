package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

func TestNewCPIOOutStage(t *testing.T) {
	stage := osbuild.NewCPIOOutStage(&osbuild.CPIOOutStageOptions{
		Filename: "initramfs.cpio",
	}, "initramfs-tree")

	data, err := json.Marshal(stage)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "org.osbuild.cpio.out",
		"inputs": {
			"tree": {
				"type": "org.osbuild.tree",
				"origin": "org.osbuild.pipeline",
				"references": ["name:initramfs-tree"]
			}
		},
		"options": {"filename": "initramfs.cpio"}
	}`, string(data))
}

func TestCPIOOutStageOptionsJSON(t *testing.T) {
	options := osbuild.CPIOOutStageOptions{
		Filename:     "archive.cpio",
		Append:       true,
		Format:       osbuild.CPIOArchiveFormatCRC,
		RootNode:     osbuild.CPIORootNodeOmit,
		Reproducible: common.ToPtr(false),
		Owner:        &osbuild.CPIOOwner{User: "0", Group: "0"},
	}

	data, err := json.Marshal(options)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"filename": "archive.cpio",
		"append": true,
		"format": "crc",
		"root-node": "omit",
		"reproducible": false,
		"owner": {"user": "0", "group": "0"}
	}`, string(data))
}
