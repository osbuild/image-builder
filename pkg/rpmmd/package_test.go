package rpmmd_test

import (
	"fmt"
	"testing"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/stretchr/testify/assert"
)

func TestChecksumString(t *testing.T) {
	assert.Equal(t, "sha256:1234567890", (&rpmmd.Checksum{Type: "sha256", Value: "1234567890"}).String())
}

var packageList = rpmmd.PackageList{
	{
		Name:    "tmux",
		Epoch:   0,
		Version: "3.3a",
		Release: "3.fc38",
		Arch:    "x86_64",
	},
	{
		Name:    "grub2",
		Epoch:   1,
		Version: "2.06",
		Release: "94.fc38",
		Arch:    "noarch",
	},
}

func TestGetPackagePackageList(t *testing.T) {
	testCases := []struct {
		packages        rpmmd.PackageList
		packageName     string
		expectedPackage *rpmmd.Package
		expectedError   error
	}{
		{
			packages:        packageList,
			packageName:     "grub2",
			expectedPackage: &packageList[1],
		},
		{
			packages:        packageList,
			packageName:     "not-a-package",
			expectedPackage: nil,
			expectedError:   fmt.Errorf("package \"not-a-package\" not found in the Package list"),
		},
		{
			packages:        rpmmd.PackageList{},
			packageName:     "tmux",
			expectedPackage: nil,
			expectedError:   fmt.Errorf("package \"tmux\" not found in the Package list"),
		},
		{
			packages:        nil,
			packageName:     "tmux",
			expectedPackage: nil,
			expectedError:   fmt.Errorf("package \"tmux\" not found in the Package list"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.packageName, func(t *testing.T) {
			pkg, err := tc.packages.Package(tc.packageName)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedPackage, pkg)
		})
	}
}

func TestPackageGetEVRA(t *testing.T) {
	assert.Equal(t, "3.3a-3.fc38.x86_64", packageList[0].EVRA())
	assert.Equal(t, "1:2.06-94.fc38.noarch", packageList[1].EVRA())
}

func TestPackageGetFullNEVRA(t *testing.T) {
	assert.Equal(t, "tmux-0:3.3a-3.fc38.x86_64", packageList[0].FullNEVRA())
	assert.Equal(t, "grub2-1:2.06-94.fc38.noarch", packageList[1].FullNEVRA())
}
