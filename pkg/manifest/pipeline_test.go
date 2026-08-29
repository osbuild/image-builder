package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/runner"
)

func TestPipelineRoleBuild(t *testing.T) {
	var mf manifest.Manifest
	manifest.NewBuild(&mf, &runner.Linux{}, nil, nil)
	assert.Equal(t, []string{"build"}, mf.BuildPipelines())
	assert.Equal(t, 0, len(mf.PayloadPipelines()))
}

func TestPipelineRoleBuildFromContainer(t *testing.T) {
	var mf manifest.Manifest
	manifest.NewBuildFromContainer(&mf, &runner.Linux{}, nil, nil)
	assert.Equal(t, []string{"build"}, mf.BuildPipelines())
	assert.Equal(t, 0, len(mf.PayloadPipelines()))
}

func TestPipelineRolePayload(t *testing.T) {
	var mf manifest.Manifest

	bp := manifest.NewBuild(&mf, &runner.Linux{}, nil, nil)

	manifest.NewXZ(bp, nil, "")
	assert.Equal(t, []string{"xz"}, mf.PayloadPipelines())
	assert.Equal(t, []string{"build"}, mf.BuildPipelines())
}

func TestPipelineRolePayloadCustomName(t *testing.T) {
	var mf manifest.Manifest

	bp := manifest.NewBuild(&mf, &runner.Linux{}, nil, nil)

	manifest.NewXZ(bp, nil, "my-xz")
	assert.Equal(t, []string{"my-xz"}, mf.PayloadPipelines())
}
