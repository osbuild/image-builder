package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/osbuild"
)

func TestErofStageJsonMinimal(t *testing.T) {
	expectedJson := `{
        "type": "org.osbuild.erofs",
        "inputs": {
                "tree": {
                        "type": "org.osbuild.tree",
                        "origin": "org.osbuild.pipeline",
                        "references": [
                                "name:input-pipeline"
                        ]
                }
        },
        "options": {
                "filename": "foo.ero"
        }
}`

	opts := osbuild.ErofsStageOptions{
		Filename: "foo.ero",
	}
	stage := osbuild.NewErofsStage(opts, "input-pipeline")
	require.NotNil(t, stage)

	json, err := json.MarshalIndent(stage, "", "        ")
	require.Nil(t, err)
	assert.Equal(t, string(json), expectedJson)
}

func TestErofStageJsonFull(t *testing.T) {
	expectedJson := `{
        "type": "org.osbuild.erofs",
        "inputs": {
                "tree": {
                        "type": "org.osbuild.tree",
                        "origin": "org.osbuild.pipeline",
                        "references": [
                                "name:input-pipeline"
                        ]
                }
        },
        "options": {
                "filename": "foo.ero",
                "source": "mount://-/",
                "exclude_paths": [
                        "boot/efi/.*",
                        "boot/initramfs-.*"
                ],
                "compression": {
                        "method": "lz4hc",
                        "level": 9
                },
                "options": [
                        "all-fragments",
                        "dedupe"
                ],
                "cluster-size": 131072
        }
}`

	opts := osbuild.ErofsStageOptions{
		Filename: "foo.ero",
		Source:   "mount://-/",
		ExcludePaths: []string{
			"boot/efi/.*",
			"boot/initramfs-.*",
		},
		Compression: &osbuild.ErofsCompression{
			Method: "lz4hc",
			Level:  common.ToPtr(9),
		},
		ExtendedOptions: []string{"all-fragments", "dedupe"},
		ClusterSize:     common.ToPtr(131072),
	}
	stage := osbuild.NewErofsStage(opts, "input-pipeline")
	require.NotNil(t, stage)

	json, err := json.MarshalIndent(stage, "", "        ")
	require.Nil(t, err)
	assert.Equal(t, string(json), expectedJson)
}
