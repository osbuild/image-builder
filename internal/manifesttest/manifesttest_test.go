package manifesttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder-cli/internal/manifesttest"
)

var fakeOsbuildManifest = `{
  "version": "2",
  "pipelines": [
    {
       "name": "noop"
    },
    {
       "name": "noop2"
    }
  ]
}`

func TestPipelineNamesFrom(t *testing.T) {
	names, err := manifesttest.PipelineNamesFrom([]byte(fakeOsbuildManifest))
	assert.NoError(t, err)
	assert.Equal(t, []string{"noop", "noop2"}, names)
}

func TestPipelineNamesFromSad(t *testing.T) {
	_, err := manifesttest.PipelineNamesFrom([]byte("bad-json"))
	assert.ErrorContains(t, err, "cannot unmarshal manifest: invalid char")

	_, err = manifesttest.PipelineNamesFrom([]byte("{}"))
	assert.ErrorContains(t, err, "cannot find any pipelines in map[]")
}
