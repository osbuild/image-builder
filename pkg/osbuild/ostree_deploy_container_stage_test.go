package osbuild

import (
	"testing"

	"github.com/google/uuid"
	"github.com/osbuild/image-builder/v73/pkg/container"
	"github.com/stretchr/testify/assert"
)

func TestOSTreeDeployContainersStageOptionsValidate(t *testing.T) {
	// options are validated first, so this doesn't necessarily need to be
	// valid, but we might change the order at some point.
	validInputs := NewContainersInputForSources([]container.Spec{
		{
			ImageID: "id-0",
			Source:  "registry.example.org/reg/img",
		},
	})

	type testCase struct {
		options OSTreeDeployContainerStageOptions
		valid   bool
		err     string
	}

	testCases := map[string]testCase{
		"empty": {
			options: OSTreeDeployContainerStageOptions{},
			valid:   false,
			err:     "osname is required",
		},
		"minimal": {
			options: OSTreeDeployContainerStageOptions{
				OsName:       "default",
				TargetImgref: "ostree-remote-registry:example.org/registry/image",
			},
			valid: true,
		},
		"no-target": {
			options: OSTreeDeployContainerStageOptions{
				OsName: "os",
			},
			valid: false,
			err:   "doesn't conform to schema",
		},
		"no-os": {
			options: OSTreeDeployContainerStageOptions{
				TargetImgref: "ostree-image-unverified-registry:example.org/registry/image",
			},
			valid: false,
			err:   "osname is required",
		},
		"bad-target": {
			options: OSTreeDeployContainerStageOptions{
				OsName:       "os",
				TargetImgref: "bad",
			},
			valid: false,
			err:   "doesn't conform to schema",
		},
		"full": {
			options: OSTreeDeployContainerStageOptions{
				OsName:       "default",
				KernelOpts:   []string{},
				TargetImgref: "ostree-image-signed:example.org/registry/image",
				Rootfs: &Rootfs{
					// defining both is redundant but not invalid
					Label: "root",
					UUID:  uuid.New().String(),
				},
				Mounts: []string{"/data"},
			},
			valid: true,
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			if tc.valid {
				assert.NoError(tc.options.validate())
				assert.NotPanics(func() { NewOSTreeDeployContainerStage(&tc.options, validInputs) })
			} else {
				assert.ErrorContains(tc.options.validate(), tc.err)
				assert.Panics(func() { NewOSTreeDeployContainerStage(&tc.options, validInputs) })
			}
		})
	}

}

func TestOSTreeDeployContainersStageInputsValidate(t *testing.T) {
	validOptions := &OSTreeDeployContainerStageOptions{
		OsName:       "default",
		TargetImgref: "ostree-remote-registry:example.org/registry/image",
	}

	type testCase struct {
		inputs OSTreeDeployContainerInputs
		valid  bool
		err    string
	}

	testCases := map[string]testCase{
		"empty": {
			inputs: OSTreeDeployContainerInputs{},
			valid:  false,
			err:    "stage requires exactly 1 input container (got nil References)",
		},
		"nil": {
			inputs: OSTreeDeployContainerInputs{
				Images: ContainersInput{
					References: nil,
				},
			},
			valid: false,
			err:   "stage requires exactly 1 input container (got nil References)",
		},
		"zero": {
			inputs: OSTreeDeployContainerInputs{
				Images: NewContainersInputForSources([]container.Spec{}),
			},
			valid: false,
			err:   "stage requires exactly 1 input container (got 0)",
		},
		"one": {
			inputs: OSTreeDeployContainerInputs{
				Images: NewContainersInputForSources([]container.Spec{
					{
						ImageID: "id-0",
						Source:  "registry.example.org/reg/img",
					},
				}),
			},
			valid: true,
		},
		"two": {
			inputs: OSTreeDeployContainerInputs{
				Images: NewContainersInputForSources([]container.Spec{
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
			valid: false,
			err:   "stage requires exactly 1 input container (got 2)",
		},
	}
	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			if tc.valid {
				assert.NoError(tc.inputs.validate())
				assert.NotPanics(func() { NewOSTreeDeployContainerStage(validOptions, tc.inputs.Images) })
			} else {
				assert.ErrorContains(tc.inputs.validate(), tc.err)
				assert.Panics(func() { NewOSTreeDeployContainerStage(validOptions, tc.inputs.Images) })
			}
		})
	}
}
