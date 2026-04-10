package flatpak_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/flatpak"
)

func TestNewRegistryFromURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name:    "Valid OCI HTTPS URI",
			uri:     "oci+https://registry.example.com",
			wantErr: false,
		},
		{
			name:    "Unsupported scheme",
			uri:     "ftp://registry.example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := flatpak.NewRegistryFromURI(tt.uri)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegistryTypeFromURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected flatpak.RegistryType
		wantErr  bool
	}{
		{
			name:     "Valid OCI HTTPS URI",
			uri:      "oci+https://registry.example.com",
			expected: flatpak.REGISTRY_TYPE_OCI,
			wantErr:  false,
		},
		{
			name:     "Unsupported scheme",
			uri:      "ftp://registry.example.com",
			expected: flatpak.REGISTRY_TYPE_UNKNOWN,
			wantErr:  false,
		},
		{
			name:     "No scheme provided",
			uri:      "registry.example.com",
			expected: flatpak.REGISTRY_TYPE_UNKNOWN,
			wantErr:  false,
		},
		{
			name:     "Invalid URI (parse error)",
			uri:      "://invalid-uri",
			expected: flatpak.REGISTRY_TYPE_UNKNOWN,
			wantErr:  true,
		},
		{
			name:     "Empty string",
			uri:      "",
			expected: flatpak.REGISTRY_TYPE_UNKNOWN,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := flatpak.RegistryTypeFromURI(tt.uri)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, got, tt.expected)
			}
		})
	}
}
