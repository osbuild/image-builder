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
	"github.com/osbuild/images/pkg/distro"
)

func TestRhel9_EC2Partitioning(t *testing.T) {
	testCases := []struct {
		distro      string
		bootSizeMiB datasizes.Size
	}{
		// x86_64
		{
			distro:      "rhel-9.2",
			bootSizeMiB: 1024,
		},
		{
			distro:      "rhel-9.3",
			bootSizeMiB: 1024,
		},
		{
			distro:      "rhel-9.4",
			bootSizeMiB: 1024,
		},
		{
			distro:      "centos-9",
			bootSizeMiB: 1024,
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
					a, err := DistroFactory(tt.distro).GetArch(arch)
					require.NoError(t, err)
					i, err := a.GetImageType(it)
					require.NoError(t, err)

					it := i.(*imageType)
					pt, err := it.getPartitionTable(&blueprint.Customizations{}, distro.ImageOptions{}, rng)
					require.NoError(t, err)

					bootSize, err := pt.GetMountpointSize("/boot")
					require.NoError(t, err)
					require.Equal(t, tt.bootSizeMiB*datasizes.MiB, bootSize)
				})

			}
		}

	}
}

func TestRhel9_DistroFactory(t *testing.T) {
	type testCase struct {
		strID    string
		expected distro.Distro
	}

	testCases := []testCase{
		{
			strID:    "rhel-90",
			expected: common.Must(New("rhel-9.0")),
		},
		{
			strID:    "rhel-9.0",
			expected: common.Must(New("rhel-9.0")),
		},
		{
			strID:    "rhel-93",
			expected: common.Must(New("rhel-9.3")),
		},
		{
			strID:    "rhel-9.3",
			expected: common.Must(New("rhel-9.3")),
		},
		{
			strID:    "rhel-910",
			expected: common.Must(New("rhel-9.10")),
		},
		{
			strID:    "rhel-9.10",
			expected: common.Must(New("rhel-9.10")),
		},
		{
			strID:    "centos-9",
			expected: common.Must(New("centos-9")),
		},
		{
			strID:    "centos-9.0",
			expected: nil,
		},
		{
			strID:    "rhel-9",
			expected: nil,
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
