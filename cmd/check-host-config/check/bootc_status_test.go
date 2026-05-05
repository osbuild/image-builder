package check_test

import (
	"errors"
	"testing"

	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/buildconfig"
	"github.com/osbuild/images/pkg/distro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootcStatusCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *buildconfig.BuildConfig
		mockExec map[string]ExecResult
		wantErr  error
	}{
		{
			name: "pass when bootc status succeeds",
			config: &buildconfig.BuildConfig{
				Options: distro.ImageOptions{
					Bootc: &distro.BootcImageOptions{},
				},
			},
			mockExec: map[string]ExecResult{
				"sudo bootc status": {Stdout: []byte("running")},
			},
		},
		{
			name: "fail when bootc status fails",
			config: &buildconfig.BuildConfig{
				Options: distro.ImageOptions{
					Bootc: &distro.BootcImageOptions{},
				},
			},
			mockExec: map[string]ExecResult{
				"sudo bootc status": {Err: errors.New("bootc not found"), Code: 1},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("bootc-status")
			require.True(t, found, "bootc-status check not found")

			err := chk.Func(chk.Meta, tt.config)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
