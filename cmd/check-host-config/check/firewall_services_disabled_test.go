package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirewallServicesDisabledCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *blueprint.FirewallCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no disabled firewall services",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when service is disabled",
			config: &blueprint.FirewallCustomization{Services: &blueprint.FirewallServicesCustomization{Disabled: []string{"badservice"}}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-service=badservice": {Stdout: []byte("no\n")},
			},
		},
		{
			name:   "fail when service is not disabled",
			config: &blueprint.FirewallCustomization{Services: &blueprint.FirewallServicesCustomization{Disabled: []string{"badservice"}}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-service=badservice": {Stdout: []byte("yes\n")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:   "fail when exec errors (exit code != 1)",
			config: &blueprint.FirewallCustomization{Services: &blueprint.FirewallServicesCustomization{Disabled: []string{"badservice"}}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-service=badservice": {Code: 2, Err: errors.New("firewall-cmd failed")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("fw-srv-disabled")
			require.True(t, found, "fw-srv-disabled check not found")
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
