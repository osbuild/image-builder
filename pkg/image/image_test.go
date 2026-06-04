package image_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func makeGetCompressionPipelineInputs() (*manifest.Manifest, manifest.Build, manifest.FilePipeline) {
	mf := manifest.New()
	build := manifest.NewBuild(&mf, &runner.Fedora{Version: 41}, []rpmmd.RepoConfig{}, nil)
	inputPipeline := manifest.NewTar(build, nil, "input")
	return &mf, build, inputPipeline
}

func TestGetCompressionPipeline(t *testing.T) {
	tests := []struct {
		name             string
		compression      manifest.Compression
		expectedPipeline string
	}{
		{"xz", manifest.CompressionXZ, "xz"},
		{"zstd", manifest.CompressionZstd, "zstd"},
		{"gzip", manifest.CompressionGzip, "gzip"},
		{"none", manifest.CompressionNone, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mf, build, inputPipeline := makeGetCompressionPipelineInputs()
			result := image.GetCompressionPipeline(tt.compression, build, inputPipeline)
			require.NotNil(t, result)
			if tt.expectedPipeline == "" {
				assert.Equal(t, inputPipeline, result)
			} else {
				assert.Equal(t, tt.expectedPipeline, result.Name())
			}

			// All compression pipelines should always be present
			payloads := mf.PayloadPipelines()
			for _, c := range manifest.CompressionTypes {
				assert.True(t, slices.Contains(payloads, string(c)),
					"expected pipeline %q in manifest payloads %v", c, payloads)
			}
		})
	}
}

func TestGetCompressionPipelinePanicsOnUnknown(t *testing.T) {
	_, build, inputPipeline := makeGetCompressionPipelineInputs()
	assert.PanicsWithValue(t, `unsupported compression type "banana"`, func() {
		image.GetCompressionPipeline(manifest.Compression("banana"), build, inputPipeline)
	})
}
