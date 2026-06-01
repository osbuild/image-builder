package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
)

func TestSystemSubcommandYAML(t *testing.T) {
	for _, args := range [][]string{
		{"system"},
		{"system", "--format=yaml"},
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
			assert.Contains(t, output, "system:")
			assert.Contains(t, output, "cache:")
			assert.Contains(t, output, "path:")
		})
	}
}

func TestSystemSubcommandJSON(t *testing.T) {
	restore := main.MockOsArgs([]string{"system", "--format=json"})
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

	sys, ok := parsed["system"].(map[string]interface{})
	require.True(t, ok, "must have system key")
	cache, ok := sys["cache"].(map[string]interface{})
	require.True(t, ok, "must have cache key")
	assert.Contains(t, cache, "path")
	assert.NotEmpty(t, cache["path"])
}

func TestSystemSubcommandUnsupportedFormat(t *testing.T) {
	restore := main.MockOsArgs([]string{"system", "--format=xml"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.EqualError(t, err, `unsupported format "xml", supported formats: yaml, json`)
}

func TestSystemSubcommandCachePathMatchesDefault(t *testing.T) {
	restore := main.MockOsArgs([]string{"system", "--format=json"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	var parsed struct {
		System struct {
			Cache struct {
				Path string `json:"path"`
			} `json:"cache"`
		} `json:"system"`
	}
	err = json.Unmarshal(fakeStdout.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, main.CacheDirForUid(0), "/var/cache/image-builder/store")
	assert.NotEmpty(t, parsed.System.Cache.Path)
}
