package generic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
)

func TestRH8_EC2Partitioning(t *testing.T) {
	testCases := []struct {
		distro             string
		aarch64bootSizeMiB datasizes.Size
	}{
		{
			distro:             "rhel-8.8",
			aarch64bootSizeMiB: 512,
		},
		{
			distro:             "rhel-8.9",
			aarch64bootSizeMiB: 512,
		},
		{
			distro:             "rhel-8.10",
			aarch64bootSizeMiB: 1024,
		},
		{
			distro:             "centos-8",
			aarch64bootSizeMiB: 1024,
		},
	}

	for _, tt := range testCases {
		for _, arch := range []string{"x86_64", "aarch64"} {
			for _, it := range []string{"ami", "ec2", "ec2-ha", "ec2-sap"} {
				// skip non-existing combos
				if strings.HasPrefix(it, "ec2") && strings.HasPrefix(tt.distro, "centos") {
					continue
				}
				if arch == "aarch64" && (it == "ec2-ha" || it == "ec2-sap") {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/%s", tt.distro, arch, it), func(t *testing.T) {
					d := DistroFactory(tt.distro)
					require.NotNil(t, d)
					a, err := d.GetArch(arch)
					require.NoError(t, err)
					i, err := a.GetImageType(it)
					require.NoError(t, err)

					it := i.(*imageType)
					pt, err := it.getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
					require.NoError(t, err)

					// x86_64 is /boot-less, check that
					if arch == "x86_64" {
						require.Nil(t, err, pt.FindMountable("/boot"))
						return
					}

					bootSize, err := pt.GetMountpointSize("/boot")
					require.NoError(t, err)
					require.Equal(t, tt.aarch64bootSizeMiB*datasizes.MiB, bootSize)
				})

			}
		}

	}
}

func TestRH8_DistroFactory(t *testing.T) {
	type testCase struct {
		strID    string
		expected distro.Distro
	}

	testCases := []testCase{
		{
			strID:    "rhel-8.0",
			expected: common.Must(New("rhel-8.0")),
		},
		{
			strID:    "rhel-80",
			expected: common.Must(New("rhel-8.0")),
		},
		{
			strID:    "rhel-8.4",
			expected: common.Must(New("rhel-8.4")),
		},
		{
			strID:    "rhel-84",
			expected: common.Must(New("rhel-8.4")),
		},
		{
			strID:    "rhel-8.10",
			expected: common.Must(New("rhel-8.10")),
		},
		{
			strID:    "rhel-810",
			expected: common.Must(New("rhel-8.10")),
		},
		{
			strID:    "centos-8",
			expected: common.Must(New("centos-8")),
		},
		{
			strID:    "centos-8.4",
			expected: nil,
		},
		{
			strID:    "rhel-8",
			expected: nil,
		},
		{
			strID:    "rhel-8.4.1",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.strID, func(t *testing.T) {
			d := DistroFactory(tc.strID)
			if tc.expected == nil {
				assert.Nil(t, d)
			} else {
				require.NotNil(t, d)
				assert.Equal(t, tc.expected.Name(), d.Name())
			}
		})
	}
}

func RH8_TestESP(t *testing.T) {
	var distros []distro.Distro
	for _, distroName := range []string{"rhel-8.8", "rhel-8.9", "rhel-8.10", "centos-8"} {
		distros = append(distros, DistroFactory(distroName))
	}

	distro_test_common.TestESP(t, distros, func(i distro.ImageType) (*disk.PartitionTable, error) {
		it := i.(*imageType)
		return it.getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
	})
}
