package osbuild_test

import (
	"testing"

	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/stretchr/testify/assert"
)

func TestGrub2IsoLegacyStageValidation(t *testing.T) {
	type testCase struct {
		options       osbuild.Grub2ISOLegacyStageOptions
		expectedError string
	}

	testCases := map[string]testCase{
		"happy": {
			options: osbuild.Grub2ISOLegacyStageOptions{
				Product: osbuild.Product{
					Name:    "whatever",
					Version: "42",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/path",
				},
				ISOLabel: "whatever-42-workstation",
			},
		},
		"no product name": {
			options: osbuild.Grub2ISOLegacyStageOptions{
				Product: osbuild.Product{
					Version: "42",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/path",
				},
				ISOLabel: "whatever-42-workstation",
			},
			expectedError: "org.osbuild.grub2.iso.legacy: product.name option is required",
		},
		"no product version": {
			options: osbuild.Grub2ISOLegacyStageOptions{
				Product: osbuild.Product{
					Name: "whatever",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/path",
				},
				ISOLabel: "whatever-42-workstation",
			},
			expectedError: "org.osbuild.grub2.iso.legacy: product.version option is required",
		},
		"no kernel dir": {
			options: osbuild.Grub2ISOLegacyStageOptions{
				Product: osbuild.Product{
					Name:    "whatever",
					Version: "13",
				},
				Kernel:   osbuild.ISOKernel{},
				ISOLabel: "whatever-42-workstation",
			},
			expectedError: "org.osbuild.grub2.iso.legacy: kernel.dir option is required",
		},
		"no isolabel": {
			options: osbuild.Grub2ISOLegacyStageOptions{
				Product: osbuild.Product{
					Name:    "whatever",
					Version: "13",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/doesnt/matter",
				},
			},
			expectedError: "org.osbuild.grub2.iso.legacy: isolabel option is required",
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			if tc.expectedError == "" {
				assert.NotPanics(t, func() { osbuild.NewGrub2ISOLegacyStage(&tc.options) })
			} else {
				assert.PanicsWithError(
					t,
					tc.expectedError,
					func() { osbuild.NewGrub2ISOLegacyStage(&tc.options) },
				)
			}
		})
	}
}
