package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   []blueprint.UserCustomization
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name:    "skip when no users",
			config:  []blueprint.UserCustomization{},
			wantErr: check.ErrCheckSkipped,
		},
		{
			name:   "pass when user exists",
			config: []blueprint.UserCustomization{{Name: "testuser"}},
			mockExec: map[string]ExecResult{
				"id testuser": {Stdout: []byte("uid=1000(testuser) gid=1000(testuser) groups=1000(testuser)\n")},
			},
		},
		{
			name:   "fail when user does not exist",
			config: []blueprint.UserCustomization{{Name: "nonexistent"}},
			mockExec: map[string]ExecResult{
				"id nonexistent": {Code: 1, Err: errors.New("id: nonexistent: no such user")},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("users")
			require.True(t, found, "users check not found")
			config := buildConfig(&blueprint.Customizations{
				User: tt.config,
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
