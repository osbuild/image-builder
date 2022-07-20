package distribution

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDistroRegistry_List(t *testing.T) {
	allDistros := []string{"rhel-8", "rhel-84", "rhel-85", "rhel-86", "rhel-9", "rhel-90", "centos-8", "centos-9"}
	notEntitledDistros := []string{"centos-8", "centos-9"}

	dr, err := LoadDistroRegistry("../../distributions")
	require.NoError(t, err)

	result := dr.Available(true).List()
	require.Len(t, result, len(allDistros))
	for _, distro := range result {
		require.Contains(t, allDistros, distro.Distribution.Name)
	}

	result = dr.Available(false).List()
	require.Len(t, result, len(notEntitledDistros))
	for _, distro := range result {
		require.Contains(t, notEntitledDistros, distro.Distribution.Name)
	}
}

func TestDistroRegistry_Get(t *testing.T) {
	dr, err := LoadDistroRegistry("../../distributions")
	require.NoError(t, err)

	result, err := dr.Available(true).Get("rhel-86")
	require.Equal(t, "rhel-86", result.Distribution.Name)
	require.Nil(t, err)

	// don't test packages, they are huge
	result.ArchX86.Packages = nil

	require.Equal(t, &DistributionFile{
		ModulePlatformID: "platform:el8",
		Distribution: DistributionItem{
			Description:      "Red Hat Enterprise Linux (RHEL) 8",
			Name:             "rhel-86",
			RestrictedAccess: false,
		},
		ArchX86: &Architecture{
			ImageTypes: []string{"aws", "gcp", "azure", "rhel-edge-commit", "rhel-edge-installer", "edge-commit", "edge-installer", "edge-container", "guest-image", "image-installer", "vsphere"},
			Repositories: []Repository{
				{
					Id:            "baseos",
					Baseurl:       "https://cdn.redhat.com/content/dist/rhel8/8.6/x86_64/baseos/os",
					Rhsm:          true,
					ImageTypeTags: nil,
				},
				{
					Id:            "appstream",
					Baseurl:       "https://cdn.redhat.com/content/dist/rhel8/8.6/x86_64/appstream/os",
					Rhsm:          true,
					ImageTypeTags: nil,
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
		},
	}, result)

	result, err = dr.Available(false).Get("toucan-42")
	require.Nil(t, result)
	require.Equal(t, DistributionNotFound, err)
}
