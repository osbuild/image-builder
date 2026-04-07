package generic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
)

func isoTestImageType() *imageType {
	return &imageType{
		arch: &architecture{
			distro: &distribution{},
		},
		ImageTypeYAML: defs.ImageTypeYAML{
			BootISO: true,
		},
		isoLabel: func(*imageType) string { return "iso-label" },
	}
}

func TestInstallerCustomizationsHonorKernelOptions(t *testing.T) {
	for _, tc := range []struct {
		imageConfig          *distro.ImageConfig
		kernelCustomizations *blueprint.KernelCustomization
		expected             []string
	}{
		{
			nil,
			nil,
			nil,
		},
		{
			nil,
			&blueprint.KernelCustomization{
				Append: "debug",
			},
			[]string{"debug"},
		},
		{
			&distro.ImageConfig{
				KernelOptions: []string{"default"},
			},
			nil,
			[]string{"default"},
		},
		{
			&distro.ImageConfig{
				KernelOptions: []string{"default"},
			},
			&blueprint.KernelCustomization{
				Append: "debug",
			},
			[]string{"default", "debug"},
		},
	} {
		it := isoTestImageType()
		it.ImageConfigYAML.ImageConfig = tc.imageConfig
		c := &blueprint.Customizations{Kernel: tc.kernelCustomizations}

		isc, err := installerCustomizations(it, c, distro.ImageOptions{})
		require.NoError(t, err)
		assert.Equal(t, tc.expected, isc.KernelOptionsAppend)
	}
}

func TestInstallerCustomizationsOverridePreview(t *testing.T) {
	for _, tc := range []struct {
		distroPreview bool
		imageOptions  distro.ImageOptions
		expected      bool
	}{
		{
			true,
			distro.ImageOptions{},
			true,
		},
		{
			false,
			distro.ImageOptions{},
			false,
		},
		{
			true,
			distro.ImageOptions{Preview: common.ToPtr(false)},
			false,
		},
		{
			false,
			distro.ImageOptions{Preview: common.ToPtr(true)},
			true,
		},
	} {
		it := isoTestImageType()
		distro := it.arch.distro.(*distribution)
		distro.Preview = tc.distroPreview

		isc, err := installerCustomizations(it, nil, tc.imageOptions)
		require.NoError(t, err)
		assert.Equal(t, tc.expected, isc.Preview)
	}

}

func TestReplaceBasictemplate(t *testing.T) {
	for _, tc := range []struct {
		input    string
		arch     arch.Arch
		expected string
	}{
		{
			input:    "$arch",
			arch:     arch.ARCH_X86_64,
			expected: arch.ARCH_X86_64.String(),
		},
		{
			input:    "foo/$arch/bar",
			arch:     arch.ARCH_AARCH64,
			expected: fmt.Sprintf("foo/%s/bar", arch.ARCH_AARCH64.String()),
		},
		{
			input:    "foo",
			arch:     arch.ARCH_AARCH64,
			expected: "foo",
		},
	} {
		assert.Equal(t, replaceBasicTemplate(tc.input, tc.arch), tc.expected)
	}
}
