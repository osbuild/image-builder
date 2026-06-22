package blueprintload_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/v73/pkg/bib/blueprintload"
)

var expectedBuildConfig = &blueprint.Blueprint{
	Customizations: &blueprint.Customizations{
		User: []blueprint.UserCustomization{
			{
				Name: "alice",
			},
		},
	},
}

var fakeConfigJSON = `{
  "customizations": {
    "user": [
      {
        "name": "alice"
      }
   ]
  }
}`

var fakeConfigToml = `
[[customizations.user]]
name = "alice"
`

func makeFakeConfig(t *testing.T, filename, content string) string {
	tmpdir := t.TempDir()
	fakeCfgPath := filepath.Join(tmpdir, filename)
	err := os.WriteFile(fakeCfgPath, []byte(content), 0644)
	assert.NoError(t, err)
	return fakeCfgPath
}

func TestReadWithFallbackUserNoConfigNoFallack(t *testing.T) {
	cfg, err := blueprintload.LoadWithFallback("")
	assert.NoError(t, err)
	assert.Equal(t, &blueprint.Blueprint{}, cfg)
}

func TestLoadWithFallbackUserProvidedConfig(t *testing.T) {
	for _, tc := range []struct {
		fname   string
		content string
	}{
		{"config.toml", fakeConfigToml},
		{"config.json", fakeConfigJSON},
	} {
		fakeUserCnfPath := makeFakeConfig(t, tc.fname, tc.content)

		cfg, err := blueprintload.LoadWithFallback(fakeUserCnfPath)
		assert.NoError(t, err)
		assert.Equal(t, expectedBuildConfig, cfg)
	}
}

func TestReadWithFallProvidedConfig(t *testing.T) {
	for _, tc := range []struct {
		fname   string
		content string
	}{
		{"config.toml", fakeConfigToml},
		{"config.json", fakeConfigJSON},
	} {
		fakeCnfPath := makeFakeConfig(t, tc.fname, tc.content)
		restore := blueprintload.MockConfigRootDir(filepath.Dir(fakeCnfPath))
		defer restore()

		cfg, err := blueprintload.LoadWithFallback("")
		assert.NoError(t, err)
		assert.Equal(t, expectedBuildConfig, cfg)
	}
}

func TestReadUserConfigErrorWrongFormat(t *testing.T) {
	for _, tc := range []struct {
		fname, content string
		expectedErr    string
	}{
		// wrong content, json in a toml file and vice-versa
		{"config.toml", fakeConfigJSON, "cannot decode"},
		{"config.json", fakeConfigToml, "cannot decode"},
	} {
		fakeCnfPath := makeFakeConfig(t, tc.fname, tc.content)

		_, err := blueprintload.LoadWithFallback(fakeCnfPath)
		assert.ErrorContains(t, err, tc.expectedErr)
	}
}

func TestReadUserConfigTwoConfigsError(t *testing.T) {
	tmpdir := t.TempDir()
	for _, fname := range []string{"config.json", "config.toml"} {
		err := os.WriteFile(filepath.Join(tmpdir, fname), nil, 0644)
		assert.NoError(t, err)
	}
	restore := blueprintload.MockConfigRootDir(tmpdir)
	defer restore()

	_, err := blueprintload.LoadWithFallback("")
	assert.ErrorContains(t, err, `found "config.json" and also "config.toml", only a single one is supported`)
}

var fakeLegacyConfigJSON = `{
  "blueprint": {
    "customizations": {
      "user": [
        {
          "name": "alice"
        }
     ]
    }
  }
}`

func TestReadLegacyJSONConfig(t *testing.T) {
	fakeUserCnfPath := makeFakeConfig(t, "config.json", fakeLegacyConfigJSON)
	cfg, err := blueprintload.LoadWithFallback(fakeUserCnfPath)
	assert.NoError(t, err)
	assert.Equal(t, expectedBuildConfig, cfg)
}

func TestTomlUnknownKeysError(t *testing.T) {
	fakeUserCnfPath := makeFakeConfig(t, "config.toml", `
[[birds]]
name = "toucan"
`)
	_, err := blueprintload.LoadWithFallback(fakeUserCnfPath)

	assert.ErrorContains(t, err, "unknown keys found: [birds birds.name]")
}

func TestJsonUnknownKeysError(t *testing.T) {
	fakeUserCnfPath := makeFakeConfig(t, "config.json", `
{
  "birds": [
	{
	  "name": "toucan"
	}
  ]
}
`)
	_, err := blueprintload.LoadWithFallback(fakeUserCnfPath)

	assert.ErrorContains(t, err, `json: unknown field "birds"`)
}

func TestReadConfigIsssue655(t *testing.T) {
	fakeUserCnfPath := makeFakeConfig(t, "config.toml", `
[[customizations.filesystem]]
mountpoint = "/"
minsize = 1000
`)

	conf, err := blueprintload.LoadWithFallback(fakeUserCnfPath)
	assert.NoError(t, err)
	assert.Equal(t, &blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    1000,
				},
			},
		},
	}, conf)
}

func TestLoadWithFallbackFromStdin(t *testing.T) {
	fakeUserCnfPath := makeFakeConfig(t, "fake-stdin", fakeConfigJSON)
	fakeStdinFp, err := os.Open(fakeUserCnfPath)
	require.NoError(t, err)
	// nolint:errcheck
	defer fakeStdinFp.Close()

	restore := blueprintload.MockOsStdin(fakeStdinFp)
	defer restore()

	cfg, err := blueprintload.LoadWithFallback("-")
	assert.NoError(t, err)
	assert.Equal(t, expectedBuildConfig, cfg)
}
