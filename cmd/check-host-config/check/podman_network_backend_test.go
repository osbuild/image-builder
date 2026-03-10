package check_test

import (
	"errors"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPodmanNetworkBackendCheck(t *testing.T) {
	tests := []struct {
		name       string
		containers []blueprint.Container
		mockExec   map[string]ExecResult
		wantErr    error
	}{
		{
			name:       "skip when no containers",
			containers: nil,
			wantErr:    check.ErrCheckSkipped,
		},
		{
			name: "pass when backends match",
			containers: []blueprint.Container{
				{Source: "registry.example.com/test"},
			},
			mockExec: map[string]ExecResult{
				"sudo podman info --format json": {
					Stdout: []byte(`{"host":{"networkBackend":"netavark"}}`),
				},
				"podman info --format json": {
					Stdout: []byte(`{"host":{"networkBackend":"netavark"}}`),
				},
			},
		},
		{
			name: "fail when backends differ",
			containers: []blueprint.Container{
				{Source: "registry.example.com/test"},
			},
			mockExec: map[string]ExecResult{
				"sudo podman info --format json": {
					Stdout: []byte(`{"host":{"networkBackend":"cni"}}`),
				},
				"podman info --format json": {
					Stdout: []byte(`{"host":{"networkBackend":"netavark"}}`),
				},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "fail when rootless podman command fails",
			containers: []blueprint.Container{
				{Source: "registry.example.com/test"},
			},
			mockExec: map[string]ExecResult{
				"sudo podman info --format json": {
					Stdout: []byte(`{"host":{"networkBackend":"netavark"}}`),
				},
				"podman info --format json": {
					Err: errors.New("podman not found"),
				},
			},
			wantErr: check.ErrCheckFailed,
		},
		{
			name: "fail when rootful podman command fails",
			containers: []blueprint.Container{
				{Source: "registry.example.com/test"},
			},
			mockExec: map[string]ExecResult{
				"sudo podman info --format json": {
					Err: errors.New("podman not found"),
				},
			},
			wantErr: check.ErrCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMockExec(t, tt.mockExec)

			chk, found := check.FindCheckByName("podman-network-backend")
			require.True(t, found, "podman-network-backend check not found")

			config := buildConfigWithBlueprint(func(bp *blueprint.Blueprint) {
				bp.Containers = tt.containers
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
