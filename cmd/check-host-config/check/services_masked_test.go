package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServicesMaskedCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *blueprint.ServicesCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no masked services",
			config:  nil,
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when service is masked",
			config: &blueprint.ServicesCustomization{Masked: []string{"test.service"}},
			mockExec: map[string]ExecResult{
				"systemctl list-unit-files --state=masked": {
					Stdout: []byte("UNIT FILE\t\t\t\t\tSTATE\n" +
						"test.service\t\t\t\t\tmasked\n" +
						"other.service\t\t\t\t\tenabled\n"),
				},
			},
		},
		{
			name:   "fail when service is not masked",
			config: &blueprint.ServicesCustomization{Masked: []string{"test.service"}},
			mockExec: map[string]ExecResult{
				"systemctl list-unit-files --state=masked": {
					Stdout: []byte("UNIT FILE\t\t\t\t\tSTATE\n" +
						"other.service\t\t\t\t\tmasked\n"),
				},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name:   "fail when list-unit-files errors",
			config: &blueprint.ServicesCustomization{Masked: []string{"test.service"}},
			mockExec: map[string]ExecResult{
				"systemctl list-unit-files --state=masked": {Code: 1, Err: errors.New("systemctl failed")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("srv-masked")
			require.True(t, found, "srv-masked check not found")
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
