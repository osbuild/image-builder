package buildconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/buildconfig"
	"github.com/osbuild/image-builder/pkg/distro"
)

func makeConfig(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)
	return tmpFile
}

func TestNew_Success(t *testing.T) {
	content := `{
		"name": "test-image",
		"blueprint": {
			"name": "bp"
		},
		"options": {
			"size": 1234
		}
	}`
	path := makeConfig(t, content)

	conf, err := buildconfig.New(path, nil)
	require.NoError(t, err)
	assert.Equal(t, &buildconfig.BuildConfig{
		Name: "test-image",
		Blueprint: &blueprint.Blueprint{
			Name: "bp",
		},
		Options: distro.ImageOptions{
			Size: uint64(1234),
		},
	}, conf)
}

func TestNew_InvalidJSON(t *testing.T) {
	content := `{invalid json}`
	path := makeConfig(t, content)

	_, err := buildconfig.New(path, nil)
	assert.ErrorContains(t, err, "cannot decode build config: ")
}

func TestNew_ExtraData(t *testing.T) {
	content := `{
		"name": "test",
		"options": {}
	} {"name": "extra", "options": {}}`
	path := makeConfig(t, content)

	_, err := buildconfig.New(path, nil)
	assert.ErrorContains(t, err, "multiple configuration objects or extra data found in ")
}

func TestNew_UnknownFields(t *testing.T) {
	content := `{
		"name": "test",
		"options": {},
		"unknown": 42
	}`
	for _, tc := range []struct {
		opts        *buildconfig.Options
		expectedErr string
	}{
		{nil, `cannot decode build config: json: unknown field "unknown"`},
		{&buildconfig.Options{AllowUnknownFields: false}, `cannot decode build config: json: unknown field "unknown"`},
		{&buildconfig.Options{AllowUnknownFields: true}, ""},
	} {
		path := makeConfig(t, content)

		_, err := buildconfig.New(path, tc.opts)
		if tc.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, tc.expectedErr)
		}
	}
}
