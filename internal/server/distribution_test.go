package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/cloudapi"
)

func TestRepositoriesForImage(t *testing.T) {
	result, err := RepositoriesForImage("../../distributions", "fedora-32", "x86_64")
	require.NoError(t, err)

	baseurl := "http://mirrors.kernel.org/fedora/releases/32/Everything/x86_64/os/"
	require.Equal(t, []cloudapi.Repository{
		{
			Baseurl:    &baseurl,
			Metalink:   nil,
			Mirrorlist: nil,
			Rhsm:       false,
		}}, result)
}

func TestRepositoriesForImageWithUnsupportedArch(t *testing.T) {
	result, err := RepositoriesForImage("../../distributions", "fedora-32", "unsupported")
	require.Nil(t, result)
	require.Error(t, err, "Architecture not supported")
}

func TestAvailableDistributions(t *testing.T) {
	result, err := AvailableDistributions("../../distributions")
	require.NoError(t, err)
	for _, distro := range result {
		require.Contains(t, []string{"fedora-32", "rhel-8", "centos-8"}, distro.Name)
	}
}

func TestArchitecturesForImage(t *testing.T) {
	result, err := ArchitecturesForImage("../../distributions", "fedora-32")
	require.NoError(t, err)
	require.Equal(t, Architectures{
		ArchitectureItem{
			Arch:       "x86_64",
			ImageTypes: []string{"ami", "vhd"},
		}}, result)
}

func TestFindPackages(t *testing.T) {
	pkgs, err := FindPackages("../../distributions", "rhel-8", "x86_64", "ssh")
	require.NoError(t, err)
	require.Greater(t, len(pkgs), 0)
	for _, p := range pkgs {
		require.Contains(t, p.Name, "ssh")
	}
}
