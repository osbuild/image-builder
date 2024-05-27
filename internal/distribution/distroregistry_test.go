package distribution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/common"
)

func TestDistroRegistry_List(t *testing.T) {
	allDistros := []string{"rhel-8", "rhel-8-nightly", "rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-88", "rhel-89", "rhel-8.10", "rhel-9", "rhel-9-nightly", "rhel-90", "rhel-91", "rhel-92", "rhel-93", "rhel-94", "centos-8", "centos-9", "fedora-37", "fedora-38", "fedora-39", "fedora-40", "fedora-41"}
	notEntitledDistros := []string{"rhel-8-nightly", "rhel-9-nightly", "centos-8", "centos-9", "fedora-37", "fedora-38", "fedora-39", "fedora-40", "fedora-41"}

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
	result.Aarch64.Packages = nil

	require.Equal(t, &DistributionFile{
		ModulePlatformID: "platform:el8",
		OscapName:        "rhel8",
		Distribution: DistributionItem{
			Description:      "Red Hat Enterprise Linux (RHEL) 8",
			Name:             "rhel-86",
			RestrictedAccess: false,
		},
		ArchX86: &Architecture{
			ImageTypes: []string{"aws", "gcp", "azure", "rhel-edge-commit", "rhel-edge-installer", "edge-commit", "edge-installer", "guest-image", "image-installer", "vsphere"},
			Repositories: []Repository{
				{
					Id:            "baseos",
					Baseurl:       common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.6/x86_64/baseos/os"),
					Rhsm:          true,
					CheckGpg:      common.ToPtr(true),
					GpgKey:        common.ToPtr(rhelGpg),
					ImageTypeTags: nil,
				},
				{
					Id:            "appstream",
					Baseurl:       common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.6/x86_64/appstream/os"),
					Rhsm:          true,
					CheckGpg:      common.ToPtr(true),
					GpgKey:        common.ToPtr(rhelGpg),
					ImageTypeTags: nil,
				},
				{
					Id:            "google-compute-engine",
					Baseurl:       common.ToPtr("https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable"),
					Rhsm:          false,
					CheckGpg:      common.ToPtr(true),
					GpgKey:        common.ToPtr(googleSdkGpg),
					ImageTypeTags: []string{"gcp"},
				},
				{
					Id:            "google-cloud-sdk",
					Baseurl:       common.ToPtr("https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64"),
					Rhsm:          false,
					CheckGpg:      common.ToPtr(true),
					GpgKey:        common.ToPtr(googleSdkGpg),
					ImageTypeTags: []string{"gcp"},
				},
			},
		},
		Aarch64: &Architecture{
			ImageTypes: []string{"aws", "guest-image", "image-installer"},
			Repositories: []Repository{
				{
					Id:            "baseos",
					Baseurl:       common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.6/aarch64/baseos/os"),
					Rhsm:          true,
					CheckGpg:      common.ToPtr(true),
					GpgKey:        common.ToPtr(rhelGpg),
					ImageTypeTags: nil,
				},
				{
					Id:            "appstream",
					Baseurl:       common.ToPtr("https://cdn.redhat.com/content/dist/rhel8/8.6/aarch64/appstream/os"),
					Rhsm:          true,
					CheckGpg:      common.ToPtr(true),
					GpgKey:        common.ToPtr(rhelGpg),
					ImageTypeTags: nil,
				},
			},
		},
	}, result)

	result, err = dr.Available(false).Get("toucan-42")
	require.Nil(t, result)
	require.Equal(t, DistributionNotFound, err)
}
