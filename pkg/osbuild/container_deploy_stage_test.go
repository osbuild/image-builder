package osbuild_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/v73/pkg/container"
	"github.com/osbuild/image-builder/v73/pkg/osbuild"
)

func TestContainersDeployStageInputs(t *testing.T) {
	inputs := osbuild.NewContainersInputForSources([]container.Spec{
		{
			ImageID: "id-0",
			Source:  "registry.example.org/reg/img",
		},
	})
	stage, err := osbuild.NewContainerDeployStage(inputs, nil)
	require.NotNil(t, stage)
	require.Nil(t, err)

	assert.Equal(t, stage.Type, "org.osbuild.container-deploy")
	assert.Equal(t, stage.Inputs.(osbuild.ContainerDeployInputs).Images, inputs)
}

func TestContainersDeployStageInputsInputsJson(t *testing.T) {
	expectedJson := `{
        "images": {
                "type": "org.osbuild.containers",
                "origin": "org.osbuild.source",
                "references": {
                        "some-id": {
                                "name": "some-local-name"
                        }
                }
        }
}`
	cdi := osbuild.ContainerDeployInputs{
		Images: osbuild.NewContainersInputForSources([]container.Spec{
			{
				ImageID:   "some-id",
				LocalName: "some-local-name",
				// hm, the values below are not actually used?
				Source:     "some-source",
				Digest:     "some-digest",
				ListDigest: "some-list-digest",
			},
		}),
	}
	json, err := json.MarshalIndent(cdi, "", "        ")
	require.Nil(t, err)
	assert.Equal(t, string(json), expectedJson)
}

func TestContainersDeployStageOptionsJson(t *testing.T) {
	expectedJson := `{
        "exclude": [
                "/sysroot",
                "/other"
        ]
}`
	cdi := osbuild.ContainerDeployOptions{
		Exclude: []string{"/sysroot", "/other"},
	}
	json, err := json.MarshalIndent(cdi, "", "        ")
	require.Nil(t, err)
	assert.Equal(t, string(json), expectedJson)
}

func TestContainersDeployStageOptionsJsonRemoveSignatures(t *testing.T) {
	expectedJson := `{
        "remove-signatures": true
}`
	cdi := osbuild.ContainerDeployOptions{
		RemoveSignatures: true,
	}
	json, err := json.MarshalIndent(cdi, "", "        ")
	require.Nil(t, err)
	assert.Equal(t, string(json), expectedJson)
}

func TestContainersDeployStageEmptyOptionsJson(t *testing.T) {
	expectedJson := `{}`
	cdi := osbuild.ContainerDeployOptions{}
	json, err := json.MarshalIndent(cdi, "", "        ")
	require.Nil(t, err)
	assert.Equal(t, string(json), expectedJson)
}

func TestContainersDeployStageInputsValidate(t *testing.T) {
	type testCase struct {
		inputs osbuild.ContainerDeployInputs
		err    string
	}

	testCases := map[string]testCase{
		"empty": {
			inputs: osbuild.ContainerDeployInputs{},
			err:    "stage requires exactly 1 input container (got nil References)",
		},
		"nil": {
			inputs: osbuild.ContainerDeployInputs{
				Images: osbuild.ContainersInput{
					References: nil,
				},
			},
			err: "stage requires exactly 1 input container (got nil References)",
		},
		"zero": {
			inputs: osbuild.ContainerDeployInputs{
				Images: osbuild.NewContainersInputForSources([]container.Spec{}),
			},
			err: "stage requires exactly 1 input container (got 0)",
		},
		"one": {
			inputs: osbuild.ContainerDeployInputs{
				Images: osbuild.NewContainersInputForSources([]container.Spec{
					{
						ImageID: "id-0",
						Source:  "registry.example.org/reg/img",
					},
				}),
			},
		},
		"two": {
			inputs: osbuild.ContainerDeployInputs{
				Images: osbuild.NewContainersInputForSources([]container.Spec{
					{
						ImageID: "id-1",
						Source:  "registry.example.org/reg/img-one",
					},
					{
						ImageID: "id-2",
						Source:  "registry.example.org/reg/img-two",
					},
				}),
			},
			err: "stage requires exactly 1 input container (got 2)",
		},
	}
	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			stage, err := osbuild.NewContainerDeployStage(tc.inputs.Images, nil)
			if tc.err == "" {
				require.NotNil(t, stage)
				require.Nil(t, err)
			} else {
				require.Nil(t, stage)
				assert.ErrorContains(t, err, tc.err)
			}
		})
	}
}
