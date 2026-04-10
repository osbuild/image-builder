package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServicesEnabledCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *blueprint.ServicesCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no enabled services",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when service is enabled",
			config: &blueprint.ServicesCustomization{Enabled: []string{"test.service"}},
			mockExec: map[string]ExecResult{
				"systemctl is-enabled test.service": {Stdout: []byte("enabled\n")},
			},
		},
		{
			name:   "fail when service is not enabled",
			config: &blueprint.ServicesCustomization{Enabled: []string{"test.service"}},
			mockExec: map[string]ExecResult{
				"systemctl is-enabled test.service": {Stdout: []byte("disabled\n")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:   "fail when exec errors",
			config: &blueprint.ServicesCustomization{Enabled: []string{"test.service"}},
			mockExec: map[string]ExecResult{
				"systemctl is-enabled test.service": {Code: 1, Err: errors.New("unit not found")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("srv-enabled")
			require.True(t, found, "srv-enabled check not found")
			config := buildConfig(&blueprint.Customizations{
				Services: tt.config,
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
