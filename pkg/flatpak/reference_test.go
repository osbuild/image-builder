package flatpak_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/flatpak"
)

func TestNewReferenceFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    flatpak.Reference
		wantErr bool
	}{
		{
			name:  "valid reference string",
			input: "app/auth-service/x86_64/main",
			want: flatpak.Reference{
				Type:       "app",
				Identifier: "auth-service",
				Arch:       "x86_64",
				Branch:     "main",
			},
			wantErr: false,
		},
		{
			name:    "too few parts",
			input:   "app/auth-service/x86_64",
			want:    flatpak.Reference{},
			wantErr: true,
		},
		{
			name:    "too many parts",
			input:   "app/auth-service/x86_64/main/extra",
			want:    flatpak.Reference{},
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    flatpak.Reference{},
			wantErr: true,
		},
		{
			name:    "valid with empty middle fields",
			input:   "type//arch/branch",
			want:    flatpak.Reference{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := flatpak.NewReferenceFromString(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "could not parse ref")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestReference_String(t *testing.T) {
	tests := []struct {
		name     string
		input    flatpak.Reference
		expected string
	}{
		{
			name: "Standard reference",
			input: flatpak.Reference{
				Type:       "app",
				Identifier: "web-server",
				Arch:       "amd64",
				Branch:     "prod",
			},
			expected: "app/web-server/amd64/prod",
		},
		{
			name: "Reference with numeric strings",
			input: flatpak.Reference{
				Type:       "oci",
				Identifier: "12345",
				Arch:       "v8",
				Branch:     "v1",
			},
			expected: "oci/12345/v8/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.String()
			assert.Equal(t, tt.expected, got, "String() should reconstruct the reference correctly")
		})
	}
}
