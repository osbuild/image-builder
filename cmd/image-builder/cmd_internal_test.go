package main

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/internal/olog"
	"github.com/osbuild/image-builder/pkg/datasizes"
	ilog "github.com/osbuild/image-builder/pkg/olog"
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

func TestVerboseFlagEnablesLogging(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want io.Writer
	}{
		{
			name: "no-flag",
			args: []string{"version"},
			want: io.Discard,
		},
		{
			name: "before-subcommand",
			args: []string{"--verbose", "version"},
			want: os.Stderr,
		},
		{
			name: "after-subcommand",
			args: []string{"version", "-v"},
			want: os.Stderr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			olog.SetDefault(nil)
			ilog.SetDefault(nil)
			defer olog.SetDefault(nil)
			defer ilog.SetDefault(nil)

			rootCmd, err := setupRootCmd()
			require.NoError(t, err)
			rootCmd.SetArgs(tc.args)
			rootCmd.SetOut(io.Discard)
			require.NoError(t, rootCmd.Execute())

			assert.Equal(t, tc.want, olog.Default().Writer())
			assert.Equal(t, tc.want, ilog.Default().Writer())
		})
	}
}
