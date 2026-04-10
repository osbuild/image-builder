package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKernelCheck(t *testing.T) {
	tests := []struct {
		name         string
		config       *blueprint.KernelCustomization
		mockExec     map[string]ExecResult
		mockReadFile map[string]ReadFileResult
		wantErr      error
	}{
		{
			name:    "skip when kernel is nil",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name: "pass with empty append and no name",
			config: &blueprint.KernelCustomization{
				Append: "",
			},
		},
		{
			name: "pass with matching append and no name",
			config: &blueprint.KernelCustomization{
				Append: "debug",
			},
			mockReadFile: map[string]ReadFileResult{
				"/proc/cmdline": {Data: []byte("BOOT_IMAGE=/vmlinuz-6.1.0 root=UUID=1234-5678 ro quiet debug")},
			},
		},
		{
			name: "pass with matching kernel name",
			config: &blueprint.KernelCustomization{
				Name: "kernel",
			},
			mockExec: map[string]ExecResult{
				"rpm -q --provides kernel": {},
			},
		},
		{
			name: "fail when rpm query fails",
			config: &blueprint.KernelCustomization{
				Name: "kernel",
			},
			mockExec: map[string]ExecResult{
				"rpm -q --provides kernel": {Code: 1, Err: errors.New("rpm command failed")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "fail when append does not match",
			config: &blueprint.KernelCustomization{
				Append: "debug",
			},
			mockReadFile: map[string]ReadFileResult{
				"/proc/cmdline": {Data: []byte("BOOT_IMAGE=/vmlinuz-6.1.0 root=UUID=1234-5678 ro quiet")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "pass with matching kernel-debug name",
			config: &blueprint.KernelCustomization{
				Name: "kernel-debug",
			},
			mockExec: map[string]ExecResult{
				"rpm -q --provides kernel-debug": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)
			installMockReadFile(t, tt.mockReadFile)

			chk, found := check.FindCheckByName("kernel")
			require.True(t, found, "Kernel Check not found")
			config := buildConfig(&blueprint.Customizations{
				Kernel: tt.config,
			})

			err := chk.Func(chk.Meta, config)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
