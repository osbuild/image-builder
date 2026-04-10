package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirewallServicesEnabledCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *blueprint.FirewallCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no enabled firewall services",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when service is enabled",
			config: &blueprint.FirewallCustomization{Services: &blueprint.FirewallServicesCustomization{Enabled: []string{"ssh"}}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-service=ssh": {Stdout: []byte("yes\n")},
			},
		},
		{
			name:   "fail when service is not enabled",
			config: &blueprint.FirewallCustomization{Services: &blueprint.FirewallServicesCustomization{Enabled: []string{"ssh"}}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-service=ssh": {Stdout: []byte("no\n")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:   "fail when exec errors",
			config: &blueprint.FirewallCustomization{Services: &blueprint.FirewallServicesCustomization{Enabled: []string{"ssh"}}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-service=ssh": {Code: 1, Err: errors.New("firewall-cmd failed")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("fw-srv-enabled")
			require.True(t, found, "fw-srv-enabled check not found")
			config := buildConfig(&blueprint.Customizations{
				Firewall: tt.config,
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
