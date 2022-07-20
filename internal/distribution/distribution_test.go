package distribution

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDistributionFile_Architecture(t *testing.T) {
	adr, err := LoadDistroRegistry("../../distributions")
	require.NoError(t, err)
	d, err := adr.Available(false).Get("centos-8")
	require.NoError(t, err)

	arch, err := d.Architecture("x86_64")
	require.NoError(t, err)

	require.Equal(t, &Architecture{
		ImageTypes: []string{"aws", "gcp", "azure", "ami", "vhd"},
		Repositories: []Repository{
			{
				Id:      "baseos",
				Baseurl: "http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/",
				Rhsm:    false,
			},
			{
				Id:      "appstream",
				Baseurl: "http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/",
				Rhsm:    false,
			},
			{
				Id:      "extras",
				Baseurl: "http://mirror.centos.org/centos/8-stream/extras/x86_64/os/",
				Rhsm:    false,
			},
			{
				Id:            "google-compute-engine",
				Baseurl:       "https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable",
				Rhsm:          false,
				ImageTypeTags: []string{"gcp"},
			},
			{
				Id:            "google-cloud-sdk",
				Baseurl:       "https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64",
				Rhsm:          false,
				ImageTypeTags: []string{"gcp"},
			},
		},
	}, arch,
	)

	arch, err = d.Architecture("unsupported")
	require.Nil(t, arch)
	require.Error(t, err, "Architecture not supported")
}

func TestFindPackages(t *testing.T) {
	pkgs, err := FindPackages("../../distributions", "centos-8", "x86_64", "vim", false)
	require.NoError(t, err)
	require.ElementsMatch(t, []Package{
		{
			Name:    "vim-minimal",
			Summary: "A minimal version of the VIM editor",
		},
		{
			Name:    "vim-common",
			Summary: "The common files needed by any version of the VIM editor",
		},
		{
			Name:    "vim-enhanced",
			Summary: "A version of the VIM editor which includes recent enhancements",
		},
		{
			Name:    "vim-X11",
			Summary: "The VIM version of the vi editor for the X Window System - GVim",
		},
		{
			Name:    "vim-filesystem",
			Summary: "VIM filesystem layout",
		},
	}, pkgs)

	pkgs, err = FindPackages("../../distributions", "rhel-84", "x86_64", "vim", true)
	require.NoError(t, err)
	require.ElementsMatch(t, []Package{
		{
			Name:    "vim-minimal",
			Summary: "A minimal version of the VIM editor",
		},
		{
			Name:    "vim-common",
			Summary: "The common files needed by any version of the VIM editor",
		},
		{
			Name:    "vim-enhanced",
			Summary: "A version of the VIM editor which includes recent enhancements",
		},
		{
			Name:    "vim-X11",
			Summary: "The VIM version of the vi editor for the X Window System - GVim",
		},
		{
			Name:    "vim-filesystem",
			Summary: "VIM filesystem layout",
		},
	}, pkgs)

	_, err = FindPackages("../../distributions", "rhel-84", "x86_64", "vim", false)
	require.Error(t, err, "users organization not entitled for distribution")
}

func TestInvalidDistribution(t *testing.T) {
	_, err := ReadDistribution("../../distributions", "none")
	require.Error(t, err, DistributionNotFound)
}

func TestDistributionFileIsRestricted(t *testing.T) {
	distsDir := "testdata/distributions"

	t.Run("distro is not restricted, has no restrictedAccess field", func(t *testing.T) {
		d, err := ReadDistribution(distsDir, "rhel-90")
		require.NoError(t, err)
		actual := d.IsRestricted()
		expected := false
		require.Equal(t, expected, actual)
	})

	t.Run("distro is not restricted, restrictedAccess field is false", func(t *testing.T) {
		d, err := ReadDistribution(distsDir, "centos-9")
		require.NoError(t, err)
		actual := d.IsRestricted()
		expected := false
		require.Equal(t, expected, actual)
	})

	t.Run("distro is restricted, restrictedAccess field is true", func(t *testing.T) {
		d, err := ReadDistribution(distsDir, "centos-8")
		require.NoError(t, err)
		actual := d.IsRestricted()
		expected := true
		require.Equal(t, expected, actual)
	})
}
