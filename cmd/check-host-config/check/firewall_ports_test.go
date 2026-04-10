package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirewallPortsCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *blueprint.FirewallCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no ports",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when port is enabled",
			config: &blueprint.FirewallCustomization{Ports: []string{"80:tcp"}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-port=80/tcp": {Stdout: []byte("yes\n")},
			},
		},
		{
			name:   "fail when port is not enabled",
			config: &blueprint.FirewallCustomization{Ports: []string{"80:tcp"}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-port=80/tcp": {Stdout: []byte("no\n")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:   "fail when exec errors",
			config: &blueprint.FirewallCustomization{Ports: []string{"80:tcp"}},
			mockExec: map[string]ExecResult{
				"sudo firewall-cmd --query-port=80/tcp": {Code: 1, Err: errors.New("firewall-cmd failed")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("fw-ports")
			require.True(t, found, "fw-ports check not found")
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
