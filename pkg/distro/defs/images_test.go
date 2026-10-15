package defs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

func isoTestImageType() *imageType {
	return &imageType{
		arch: &architecture{
			distro: &distribution{},
		},
		bootISO:  true,
		isoLabel: "iso-label",
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
		if ic := tc.imageConfig; ic != nil {
			it.imageConfig = *ic
		}
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

func TestInstallerCustomizationsPackages(t *testing.T) {
	it := isoTestImageType()
	c := &blueprint.Customizations{
		Installer: &blueprint.InstallerCustomization{
			Packages: []blueprint.Package{
				{Name: "driver-pkg"},
				{Name: "versioned-pkg", Version: "1.0"},
			},
		},
	}

	isc, err := installerCustomizations(it, c, distro.ImageOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"driver-pkg", "versioned-pkg-1.0"}, isc.Packages)
}

func TestInstallerCustomizationsPackagesEmpty(t *testing.T) {
	it := isoTestImageType()
	c := &blueprint.Customizations{}

	isc, err := installerCustomizations(it, c, distro.ImageOptions{})
	require.NoError(t, err)
	assert.Nil(t, isc.Packages)
}

func TestISOCustomizationsReplacesArchInVolumeID(t *testing.T) {
	for _, tc := range []struct {
		volumeID      string
		arch          arch.Arch
		expectedLabel string
	}{
		{
			volumeID:      "MyISO-$arch",
			arch:          arch.ARCH_X86_64,
			expectedLabel: "MyISO-x86_64",
		},
		{
			volumeID:      "MyISO-$arch",
			arch:          arch.ARCH_AARCH64,
			expectedLabel: "MyISO-aarch64",
		},
		{
			volumeID:      "MyISO-no-template",
			arch:          arch.ARCH_X86_64,
			expectedLabel: "MyISO-no-template",
		},
	} {
		t.Run(fmt.Sprintf("%s/%s", tc.volumeID, tc.arch), func(t *testing.T) {
			it := isoTestImageType()
			c := &blueprint.Customizations{
				ISO: &blueprint.ISOCustomization{
					VolumeID: tc.volumeID,
				},
			}
			isc, err := isoCustomizations(it, c, tc.arch)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLabel, isc.Label)
		})
	}

	t.Run("nil-customizations-preserves-default", func(t *testing.T) {
		it := isoTestImageType()
		isc, err := isoCustomizations(it, nil, arch.ARCH_X86_64)
		require.NoError(t, err)
		assert.Equal(t, "iso-label", isc.Label)
	})
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

func TestDefaultTargetOverride(t *testing.T) {
	for _, tc := range []struct {
		confDefaultTarget string
		bpDefaultTarget   string
		expected          string
	}{
		{
			confDefaultTarget: "multi-user.target",
			bpDefaultTarget:   "graphical.target",
			expected:          "graphical.target",
		},
		{
			confDefaultTarget: "multi-user.target",
			bpDefaultTarget:   "",
			expected:          "multi-user.target",
		},
	} {
		t.Run(fmt.Sprintf("imageConfig: %s, blueprint: %s", tc.confDefaultTarget, tc.bpDefaultTarget), func(t *testing.T) {
			it := isoTestImageType()
			it.imageConfig.DefaultTarget = &tc.confDefaultTarget

			bp := &blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Services: &blueprint.ServicesCustomization{
						DefaultTarget: tc.bpDefaultTarget,
					},
				},
			}

			osc, err := osCustomizations(
				it,
				rpmmd.PackageSet{},
				distro.ImageOptions{},
				nil,
				bp,
			)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, osc.DefaultTarget)
		})
	}
}
