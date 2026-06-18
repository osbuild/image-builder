package image

import (
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

var (
	AddBuildBootstrapPipelines = addBuildBootstrapPipelines
)

func MockManifestNewBuild(new func(m *manifest.Manifest, runner runner.Runner, repos []rpmmd.RepoConfig, opts *manifest.BuildOptions) manifest.Build) (restore func()) {
	saved := manifestNewBuild
	manifestNewBuild = new
	return func() {
		manifestNewBuild = saved
	}
}
