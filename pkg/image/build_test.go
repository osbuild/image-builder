package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild/manifesttest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var (
	fakeDigest = rpmmd.Checksum{
		Type:  "sha256",
		Value: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	fakeCntDigest = fakeDigest.String()
	fakeCntSpecs  = map[string][]container.Spec{
		"bootstrap-buildroot": []container.Spec{{Source: "some-src", Digest: fakeCntDigest, ImageID: fakeCntDigest}},
	}
)

func TestNewBuildWithExperimentalOverride(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		env                        string
		manifestDistroBootstrapRef string

		expectBootstrap bool
	}{
		{"no-buildroot-env", "", "", false},
		{"buildroot-env-set", "bootstrap=ghcr.io/ondrejbudai/cool:stuff", "", true},
		{"manifest-opt-set", "", "ghcr.io/ondrej/cool:stuff", true},
		// XXX: add test that ensures that env wins
		{"env-set-manifest-set", "ghcr.io/from-env", "ghcr.io/from-manifest", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IMAGE_BUILDER_EXPERIMENTAL", tc.env)
			mf := manifest.New()
			runner := &runner.Fedora{Version: 42}
			mf.DistroBootstrapRef = tc.manifestDistroBootstrapRef
			buildIf := image.AddBuildBootstrapPipelines(&mf, runner, nil, nil)
			require.NotNil(t, buildIf)

			b, err := mf.Serialize(nil, fakeCntSpecs, nil, nil, nil)
			assert.NoError(t, err)
			pipelines, err := manifesttest.PipelineNamesFrom(b)
			assert.NoError(t, err)
			if tc.expectBootstrap {
				assert.Equal(t, []string{"bootstrap-buildroot", "build"}, pipelines)
				// XXX: this is very crude
				assert.Contains(t, string(b), `"name":"build","build":"name:bootstrap-buildroot"`)
			} else {
				assert.Equal(t, []string{"build"}, pipelines)
			}
		})
	}
}
