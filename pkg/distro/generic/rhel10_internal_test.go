package generic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/distro/distro_test_common"
)

func TestRH10DistroFactory(t *testing.T) {
	type testCase struct {
		strID    string
		expected distro.Distro
	}

	testCases := []testCase{
		{
			strID:    "rhel-100",
			expected: nil,
		},
		{
			strID:    "rhel-10.0",
			expected: common.Must(New("rhel-10.0")),
		},
		{
			strID:    "rhel-103",
			expected: nil,
		},
		{
			strID:    "rhel-10.3",
			expected: common.Must(New("rhel-10.3")),
		},
		{
			strID:    "rhel-1010",
			expected: nil,
		},
		{
			strID:    "rhel-10.10",
			expected: common.Must(New("rhel-10.10")),
		},
		{
			strID:    "centos-10",
			expected: common.Must(New("centos-10")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.strID, func(t *testing.T) {
			d := DistroFactory(tc.strID)
			if tc.expected == nil {
				assert.Nil(t, d)
			} else {
				assert.NotNil(t, d)
				assert.Equal(t, tc.expected.Name(), d.Name())
			}
		})
	}
}

func TestRhel10_NoBootPartition(t *testing.T) {
	for _, distroName := range []string{"rhel-10.0", "centos-10"} {
		dist := DistroFactory(distroName)
		require.NotNil(t, dist, distroName)
		for _, archName := range dist.ListArches() {
			arch, err := dist.GetArch(archName)
			assert.NoError(t, err)
			for _, imgTypeName := range arch.ListImageTypes() {
				imgType, err := arch.GetImageType(imgTypeName)
				assert.NoError(t, err)
				it := imgType.(*imageType)
				if it.ImageTypeYAML.PartitionTables == nil {
					continue
				}
				switch it.Name() {
				case "azure-rhui", "azure-sap-rhui", "azure-sapapps-rhui", "azure", "oci":
					// Azure and OCI RHEL internal image type PT is by default
					// LVM-based and we do not support /boot on LVM, so it must
					// be on a separate partition.
					continue
				}
				pt, err := it.getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
				assert.NoError(t, err)
				_, err = pt.GetMountpointSize("/boot")
				require.EqualError(t, err, "cannot find mountpoint /boot")
			}
		}
	}
}

func TestESP(t *testing.T) {
	var distros []distro.Distro
	for _, distroName := range []string{"rhel-10.0", "centos-10"} {
		distros = append(distros, common.Must(New(distroName)))
	}

	distro_test_common.TestESP(t, distros, func(i distro.ImageType) (*disk.PartitionTable, error) {
		it := i.(*imageType)
		return it.getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
	})
}

// ec2-cvm is UKI-based: its kernels live in the ESP, so the 512 MiB ESP of
// its base partition table must survive a disk customization that doesn't
// mention /boot/efi.
func TestRhel10DiskCustomizationKeepsESPSize(t *testing.T) {
	dist := DistroFactory("rhel-10.0")
	require.NotNil(t, dist)
	a, err := dist.GetArch("x86_64")
	require.NoError(t, err)
	imgType, err := a.GetImageType("ec2-cvm")
	require.NoError(t, err)
	it := imgType.(*imageType)

	customizations := &blueprint.Customizations{
		Disk: &blueprint.DiskCustomization{
			Partitions: []blueprint.PartitionCustomization{
				{
					Type: "plain",
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: "/",
						FSType:     "xfs",
					},
				},
			},
		},
	}
	pt, err := it.getPartitionTable(customizations, distro.ImageOptions{}, rng)
	require.NoError(t, err)
	assert.Equal(t, datasizes.Size(512*datasizes.MiB), pt.ESPSize())
}
