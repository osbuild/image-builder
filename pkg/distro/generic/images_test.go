package generic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/rpmmd"
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

func diskTestImageType() *imageType {
	return &imageType{
		arch: &architecture{
			distro: &distribution{},
		},
		ImageTypeYAML: defs.ImageTypeYAML{},
	}
}

func ostreeTestImageType() *imageType {
	it := diskTestImageType()
	it.ImageTypeYAML.OSTree.Ref = "rhel/9/x86_64/edge"
	return it
}

func TestOSCustomizationsPodmanDefaultNetBackend(t *testing.T) {
	netavark := container.NetworkBackendNetavark

	tests := []struct {
		name         string
		imageType    func() *imageType
		backend      *container.NetworkBackend
		containers   []container.SourceSpec
		expectFile   bool
		expectedPath string
		expectedVal  string
	}{
		{
			name:      "disk: backend set with containers creates file",
			imageType: diskTestImageType,
			backend:   &netavark,
			containers: []container.SourceSpec{
				{Source: "registry.example.com/test:latest"},
			},
			expectFile:   true,
			expectedPath: "/var/lib/containers/storage/defaultNetworkBackend",
			expectedVal:  "netavark",
		},
		{
			name:      "disk: nil backend with containers does not create file",
			imageType: diskTestImageType,
			backend:   nil,
			containers: []container.SourceSpec{
				{Source: "registry.example.com/test:latest"},
			},
			expectFile: false,
		},
		{
			name:       "disk: backend set without containers does not create file",
			imageType:  diskTestImageType,
			backend:    &netavark,
			containers: nil,
			expectFile: false,
		},
		{
			name:       "disk: nil backend without containers does not create file",
			imageType:  diskTestImageType,
			backend:    nil,
			containers: nil,
			expectFile: false,
		},
		{
			name:      "ostree: backend set with containers creates file in relocated path",
			imageType: ostreeTestImageType,
			backend:   &netavark,
			containers: []container.SourceSpec{
				{Source: "registry.example.com/test:latest"},
			},
			expectFile:   true,
			expectedPath: "/usr/share/containers/storage/defaultNetworkBackend",
			expectedVal:  "netavark",
		},
		{
			name:      "ostree: nil backend with containers does not create file",
			imageType: ostreeTestImageType,
			backend:   nil,
			containers: []container.SourceSpec{
				{Source: "registry.example.com/test:latest"},
			},
			expectFile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := tt.imageType()
			it.ImageConfigYAML.ImageConfig = &distro.ImageConfig{
				PodmanDefaultNetBackend: tt.backend,
			}

			bp := &blueprint.Blueprint{}
			osc, err := osCustomizations(it, rpmmd.PackageSet{}, distro.ImageOptions{}, tt.containers, bp)
			require.NoError(t, err)

			if !tt.expectFile {
				for _, f := range osc.Files {
					assert.NotContains(t, f.Path(), "defaultNetworkBackend",
						"unexpected defaultNetworkBackend file found at %s", f.Path())
				}
				return
			}

			var found bool
			for _, f := range osc.Files {
				if f.Path() == tt.expectedPath {
					found = true
					assert.Equal(t, []byte(tt.expectedVal), f.Data())
					break
				}
			}
			assert.True(t, found, "expected file at %s", tt.expectedPath)
		})
	}
}
