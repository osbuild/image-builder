package generic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
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
				if it.Name() == "azure-rhui" || it.Name() == "azure-sap-rhui" || it.Name() == "azure-sapapps-rhui" || it.Name() == "azure" {
					// Azure RHEL internal image type PT is by default LVM-based
					// and we do not support /boot on LVM, so it must be on a separate partition.
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
