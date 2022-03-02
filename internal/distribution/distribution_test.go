package distribution

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoriesForArch(t *testing.T) {
	result, err := RepositoriesForArch("../../distributions", "centos-8", "x86_64")
	require.NoError(t, err)

	require.Equal(t, []Repository{
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
	}, result)
}

func TestRepositoriesForArchWithUnsupportedArch(t *testing.T) {
	result, err := RepositoriesForArch("../../distributions", "centos-8", "unsupported")
	require.Nil(t, result)
	require.Error(t, err, "Architecture not supported")
}

func TestAvailableDistributions(t *testing.T) {
	result, err := AvailableDistributions("../../distributions")
	require.NoError(t, err)
	for _, distro := range result {
		require.Contains(t, []string{"rhel-84", "rhel-85", "centos-8", "centos-9"}, distro.Name)
	}
}

func TestFindPackages(t *testing.T) {
	pkgs, err := FindPackages("../../distributions", "centos-8", "x86_64", "ssh")
	require.NoError(t, err)
	require.Greater(t, len(pkgs), 0)
	for _, p := range pkgs {
		require.Contains(t, p.Name, "ssh")
	}
}

func TestInvalidDistribution(t *testing.T) {
	_, err := ReadDistribution("../../distributions", "none")
	require.Error(t, err, DistributionNotFound)
}
