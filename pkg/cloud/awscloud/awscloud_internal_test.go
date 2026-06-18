package awscloud

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/stretchr/testify/assert"
)

func TestEc2BootMode(t *testing.T) {
	testCases := []struct {
		name     string
		bootMode *platform.BootMode
		expected ec2types.BootModeValues
		err      bool
	}{
		{
			name:     "nil",
			bootMode: nil,
			expected: ec2types.BootModeValues(""),
		},
		{
			name:     "boot-mode-legacy",
			bootMode: common.ToPtr(platform.BOOT_LEGACY),
			expected: ec2types.BootModeValuesLegacyBios,
		},
		{
			name:     "boot-mode-uefi",
			bootMode: common.ToPtr(platform.BOOT_UEFI),
			expected: ec2types.BootModeValuesUefi,
		},
		{
			name:     "boot-mode-hybrid",
			bootMode: common.ToPtr(platform.BOOT_HYBRID),
			expected: ec2types.BootModeValuesUefiPreferred,
		},
		{
			name:     "boot-mode-invalid",
			bootMode: common.ToPtr(platform.BootMode(123456)), // Invalid boot mode
			err:      true,
		},
		{
			name:     "boot-mode-none",
			bootMode: common.ToPtr(platform.BOOT_NONE),
			err:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := ec2BootMode(tc.bootMode)
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			}
		})
	}
}
