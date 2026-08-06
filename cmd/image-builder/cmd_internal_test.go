package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/datasizes"
)

func TestManifestImageSizeFlag(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected datasizes.Size
	}{
		{
			name:     "bytes",
			input:    "1073741824",
			expected: datasizes.Size(datasizes.GiB),
		},
		{
			name:     "with-unit",
			input:    "1 GiB",
			expected: datasizes.Size(datasizes.GiB),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestCmd, err := setupManifestCmd()
			require.NoError(t, err)
			require.NoError(t, manifestCmd.ParseFlags([]string{"--image-size", tc.input}))

			var imageSize datasizes.Size
			require.NoError(t, manifestCmd.Flags().GetText("image-size", &imageSize))
			assert.Equal(t, tc.expected, imageSize)
		})
	}
}

func TestManifestCommandDocumentsImageSizeUsage(t *testing.T) {
	manifestCmd, err := setupManifestCmd()
	require.NoError(t, err)

	assert.Contains(t, manifestCmd.Long, "--image-size")
	assert.Contains(t, manifestCmd.Example, `--image-size "1 GiB"`)
}
