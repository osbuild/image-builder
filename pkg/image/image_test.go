package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func makeGetCompressionPipelineInputs() (manifest.Build, manifest.FilePipeline) {
	mf := manifest.New()
	build := manifest.NewBuild(&mf, &runner.Fedora{Version: 41}, []rpmmd.RepoConfig{}, nil)
	inputPipeline := manifest.NewTar(build, nil, "input")
	return build, inputPipeline
}

func TestGetCompressionPipeline(t *testing.T) {
	tests := []struct {
		name             string
		compression      string
		expectedPipeline string
	}{
		{"xz", "xz", "xz"},
		{"zstd", "zstd", "zstd"},
		{"gzip", "gzip", "gzip"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build, inputPipeline := makeGetCompressionPipelineInputs()
			result := image.GetCompressionPipeline(tt.compression, build, inputPipeline)
			require.NotNil(t, result)
			if tt.expectedPipeline == "" {
				assert.Equal(t, inputPipeline, result)
			} else {
				assert.Equal(t, tt.expectedPipeline, result.Name())
			}
		})
	}
}

func TestGetCompressionPipelinePanicsOnUnknown(t *testing.T) {
	build, inputPipeline := makeGetCompressionPipelineInputs()
	assert.PanicsWithValue(t, `unsupported compression type "banana"`, func() {
		image.GetCompressionPipeline("banana", build, inputPipeline)
	})
}
