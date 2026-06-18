package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/image-builder/cmd/image-builder"
)

func runSystemRaw(t *testing.T) map[string]interface{} {
	t.Helper()
	restore := main.MockOsArgs([]string{"system", "--format=json"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(fakeStdout.Bytes(), &parsed)
	require.NoError(t, err)
	return parsed
}

func cacheFromRaw(t *testing.T, raw map[string]interface{}) map[string]interface{} {
	t.Helper()
	sys, ok := raw["system"].(map[string]interface{})
	require.True(t, ok)
	cache, ok := sys["cache"].(map[string]interface{})
	require.True(t, ok)
	return cache
}

func TestSystemSubcommandYAML(t *testing.T) {
	restore := main.MockDirSize(func(string) (int64, error) {
		return 4096, nil
	})
	defer restore()

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
			assert.Contains(t, output, "size:")
			assert.Contains(t, output, "max-size:")
		})
	}
}

func TestSystemSubcommandJSON(t *testing.T) {
	restore := main.MockDirSize(func(string) (int64, error) {
		return 12345, nil
	})
	defer restore()

	raw := runSystemRaw(t)
	cache := cacheFromRaw(t, raw)
	assert.NotEmpty(t, cache["path"])
	assert.Equal(t, float64(12345), cache["size"])
	assert.Contains(t, cache, "max-size")
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

func TestSystemSubcommandCacheMaxSizeFromFile(t *testing.T) {
	xdgCache := t.TempDir()
	cacheDir := filepath.Join(xdgCache, "image-builder", "store")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "cache.size"), []byte("21474836480\n"), 0600))

	restore := main.MockGetCacheDir(func() string {
		return cacheDir
	})
	defer restore()

	restore = main.MockDirSize(func(string) (int64, error) {
		return 1500, nil
	})
	defer restore()

	raw := runSystemRaw(t)
	cache := cacheFromRaw(t, raw)
	assert.Equal(t, float64(21474836480), cache["max-size"])
}

func TestSystemSubcommandCacheMaxSizeZeroIsUnlimited(t *testing.T) {
	xdgCache := t.TempDir()
	cacheDir := filepath.Join(xdgCache, "image-builder", "store")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "cache.size"), []byte("0\n"), 0600))

	restore := main.MockGetCacheDir(func() string {
		return cacheDir
	})
	defer restore()

	restore = main.MockDirSize(func(string) (int64, error) {
		return 0, nil
	})
	defer restore()

	raw := runSystemRaw(t)
	cache := cacheFromRaw(t, raw)
	assert.Equal(t, "unlimited", cache["max-size"])
}

func TestSystemSubcommandCacheMaxSizeNoFileIsUnknown(t *testing.T) {
	restore := main.MockDirSize(func(string) (int64, error) {
		return 0, nil
	})
	defer restore()

	raw := runSystemRaw(t)
	cache := cacheFromRaw(t, raw)
	assert.Equal(t, "unknown", cache["max-size"])
}

func TestSystemSubcommandCacheDirNotExist(t *testing.T) {
	restore := main.MockDirSize(func(string) (int64, error) {
		return 0, os.ErrNotExist
	})
	defer restore()

	raw := runSystemRaw(t)
	cache := cacheFromRaw(t, raw)
	assert.Equal(t, float64(0), cache["size"])
	assert.Equal(t, "unknown", cache["max-size"])
}
