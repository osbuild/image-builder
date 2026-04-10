package image

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func TestNewGCETarPipeline(t *testing.T) {
	for _, tc := range []struct {
		imgFilename       string
		expectedTransform string
	}{
		{"disk.raw", ""},
		{"foo.img", `s/foo\.img/disk.raw/`},
	} {
		var repos []rpmmd.RepoConfig
		m := &manifest.Manifest{}
		runner := &runner.Fedora{}

		buildPipeline := manifest.NewBuild(m, runner, repos, nil)
		buildPipeline.Checkpoint()

		imgPipeline := manifest.NewRawImage(buildPipeline, nil, manifest.DiskCustomizations{})
		imgPipeline.SetFilename(tc.imgFilename)

		tar := newGCETarPipelineForImg(buildPipeline, imgPipeline, "my-test")
		assert.NotNil(t, tar)
		assert.Equal(t, tar.Format, osbuild.TarArchiveFormatOldgnu)
		assert.Equal(t, tar.RootNode, osbuild.TarRootNodeOmit)
		assert.Equal(t, *tar.ACLs, false)
		assert.Equal(t, *tar.SELinux, false)
		assert.Equal(t, *tar.Xattrs, false)
		assert.Equal(t, tar.Transform, tc.expectedTransform)

	}
}
