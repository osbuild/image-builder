package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/image-builder/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTargetCheck(t *testing.T) {
	for _, tc := range []struct {
		name     string
		config   *blueprint.ServicesCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no default target is specified",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when correct default target is set",
			config: &blueprint.ServicesCustomization{DefaultTarget: "graphical.target"},
			mockExec: map[string]ExecResult{
				"systemctl get-default": {Stdout: []byte("graphical.target")},
			},
		},
		{
			name:   "fail when different default target is set",
			config: &blueprint.ServicesCustomization{DefaultTarget: "graphical.target"},
			mockExec: map[string]ExecResult{
				"systemctl get-default": {Stdout: []byte("multi-user.target")},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:   "fail when exec errors",
			config: &blueprint.ServicesCustomization{DefaultTarget: "graphical.target"},
			mockExec: map[string]ExecResult{
				"systemctl get-default": {Code: 1, Err: errors.New("Failed to get default target")},
			},
			wantErr: check.ErrCheckFailed,
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			installMockExec(t, tc.mockExec)

			chk, found := check.FindCheckByName("default-target")
			require.True(t, found, "default-target check not found")
			config := buildConfig(&blueprint.Customizations{
				Services: tc.config,
			})

			err := chk.Func(chk.Meta, config)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
