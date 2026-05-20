package image

import (
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
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
