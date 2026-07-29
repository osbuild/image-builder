package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/osbuild/image-builder/pkg/manifest"
)

func TestCompressionConfigUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected manifest.CompressionConfig
	}{
		{
			name:  "xz",
			input: "default: xz",
			expected: manifest.CompressionConfig{
				Default: manifest.CompressionXZ,
			},
		},
		{
			name:  "zstd",
			input: "default: zstd",
			expected: manifest.CompressionConfig{
				Default: manifest.CompressionZstd,
			},
		},
		{
			name:  "gzip",
			input: "default: gzip",
			expected: manifest.CompressionConfig{
				Default: manifest.CompressionGzip,
			},
		},
		{
			name:  "none",
			input: "default: none",
			expected: manifest.CompressionConfig{
				Default: manifest.CompressionNone,
			},
		},
		{
			name:  "empty",
			input: "{}",
			expected: manifest.CompressionConfig{
				Default: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cc manifest.CompressionConfig
			err := yaml.Unmarshal([]byte(tt.input), &cc)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cc)
		})
	}
}
