package osbuild_test

import (
	"testing"

	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/stretchr/testify/assert"
)

func TestGrubIsoStageValidation(t *testing.T) {
	type testCase struct {
		options       osbuild.GrubISOStageOptions
		expectedError string
	}

	testCases := map[string]testCase{
		"happy": {
			options: osbuild.GrubISOStageOptions{
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
			options: osbuild.GrubISOStageOptions{
				Product: osbuild.Product{
					Version: "42",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/path",
				},
				ISOLabel: "whatever-42-workstation",
			},
			expectedError: "org.osbuild.grub2.iso: product.name option is required",
		},
		"no product version": {
			options: osbuild.GrubISOStageOptions{
				Product: osbuild.Product{
					Name: "whatever",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/path",
				},
				ISOLabel: "whatever-42-workstation",
			},
			expectedError: "org.osbuild.grub2.iso: product.version option is required",
		},
		"no kernel dir": {
			options: osbuild.GrubISOStageOptions{
				Product: osbuild.Product{
					Name:    "whatever",
					Version: "13",
				},
				Kernel:   osbuild.ISOKernel{},
				ISOLabel: "whatever-42-workstation",
			},
			expectedError: "org.osbuild.grub2.iso: kernel.dir option is required",
		},
		"no isolabel": {
			options: osbuild.GrubISOStageOptions{
				Product: osbuild.Product{
					Name:    "whatever",
					Version: "13",
				},
				Kernel: osbuild.ISOKernel{
					Dir: "/doesnt/matter",
				},
			},
			expectedError: "org.osbuild.grub2.iso: isolabel option is required",
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			if tc.expectedError == "" {
				assert.NotPanics(t, func() { osbuild.NewGrubISOStage(&tc.options) })
			} else {
				assert.PanicsWithError(
					t,
					tc.expectedError,
					func() { osbuild.NewGrubISOStage(&tc.options) },
				)
			}
		})
	}
}
