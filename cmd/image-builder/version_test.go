package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func TestVersionFlagDeprecated(t *testing.T) {
	restore := main.MockOsArgs([]string{"--version"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	var fakeStderr bytes.Buffer
	restore = main.MockOsStderr(&fakeStderr)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	output := fakeStdout.String()
	assert.Contains(t, output, "image-builder:")
	assert.Contains(t, output, "version:")
	assert.Contains(t, output, "commit:")
	assert.Contains(t, output, "dependencies:")
	assert.NotContains(t, output, "deprecated")

	assert.Contains(t, fakeStderr.String(), "deprecated")
}

func TestVersionSubcommandYAML(t *testing.T) {
	for _, args := range [][]string{
		{"version"},
		{"version", "--format=yaml"},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			restore := main.MockOsArgs(args)
			defer restore()

			var fakeStdout bytes.Buffer
			restore = main.MockOsStdout(&fakeStdout)
			defer restore()

			err := main.Run()
			require.NoError(t, err)

			output := fakeStdout.String()
			assert.Contains(t, output, "image-builder:")
			assert.Contains(t, output, "version:")
			assert.Contains(t, output, "commit:")
			assert.Contains(t, output, "dependencies:")
		})
	}
}

func TestVersionSubcommandJSON(t *testing.T) {
	restore := main.MockOsArgs([]string{"version", "--format=json"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	output := fakeStdout.String()

	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err, "output must be valid JSON")

	ib, ok := parsed["image-builder"].(map[string]interface{})
	require.True(t, ok, "must have image-builder key")
	assert.Contains(t, ib, "version")
	assert.Contains(t, ib, "commit")

	deps, ok := ib["dependencies"].(map[string]interface{})
	require.True(t, ok, "must have dependencies key")
	assert.Contains(t, deps, "images")
	assert.Contains(t, deps, "osbuild")
}

func TestVersionSubcommandUnsupportedFormat(t *testing.T) {
	restore := main.MockOsArgs([]string{"version", "--format=xml"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.EqualError(t, err, `unsupported format "xml", supported formats: yaml, json`)
}
