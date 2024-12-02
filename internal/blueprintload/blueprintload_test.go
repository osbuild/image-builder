package blueprintload_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/blueprint"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
)

var testBlueprintJSON = `{
  "customizations": {
    "user": [
      {
	"name": "alice"
      }
    ]
  }
}`

var testBlueprintTOML = `
[[customizations.user]]
name = "alice"
`

var expectedBlueprint = &blueprint.Blueprint{
	Customizations: &blueprint.Customizations{
		User: []blueprint.UserCustomization{
			{
				Name: "alice",
			},
		},
	},
}

func makeTestBlueprint(t *testing.T, name, content string) string {
	tmpdir := t.TempDir()
	blueprintPath := filepath.Join(tmpdir, name)
	err := os.WriteFile(blueprintPath, []byte(content), 0644)
	assert.NoError(t, err)
	return blueprintPath
}

func TestBlueprintLoadJSON(t *testing.T) {
	for _, tc := range []struct {
		fname   string
		content string

		expectedBp    *blueprint.Blueprint
		expectedError string
	}{
		{"bp.json", testBlueprintJSON, expectedBlueprint, ""},
		{"bp.toml", testBlueprintTOML, expectedBlueprint, ""},
		{"bp.toml", "wrong-content", nil, `cannot decode .*/bp.toml": toml: `},
		{"bp.json", "wrong-content", nil, `cannot decode .*/bp.json": invalid `},
		{"bp", "wrong-content", nil, `unsupported file extension for "/.*/bp"`},
	} {
		blueprintPath := makeTestBlueprint(t, tc.fname, tc.content)
		bp, err := blueprintload.Load(blueprintPath)
		if tc.expectedError == "" {
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedBp, bp)
		} else {
			assert.NotNil(t, err)
			assert.Regexp(t, tc.expectedError, err.Error())
		}
	}
}
