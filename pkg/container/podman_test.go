package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/container"
)

func TestGenDefaultNetworkBackendFile(t *testing.T) {
	tests := []struct {
		name            string
		storagePath     string
		backend         container.NetworkBackend
		expectedPath    string
		expectedContent string
	}{
		{
			name:            "netavark backend with default path",
			storagePath:     "",
			backend:         container.NetworkBackendNetavark,
			expectedPath:    "/var/lib/containers/storage/defaultNetworkBackend",
			expectedContent: "netavark",
		},
		{
			name:            "cni backend with default path",
			storagePath:     "",
			backend:         container.NetworkBackendCNI,
			expectedPath:    "/var/lib/containers/storage/defaultNetworkBackend",
			expectedContent: "cni",
		},
		{
			name:            "netavark backend with custom storage path",
			storagePath:     "/usr/share/containers/storage",
			backend:         container.NetworkBackendNetavark,
			expectedPath:    "/usr/share/containers/storage/defaultNetworkBackend",
			expectedContent: "netavark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := container.GenDefaultNetworkBackendFile(tt.storagePath, tt.backend)
			require.NoError(t, err)
			require.NotNil(t, file)
			assert.Equal(t, tt.expectedPath, file.Path())
			assert.Equal(t, []byte(tt.expectedContent), file.Data())
		})
	}
}
