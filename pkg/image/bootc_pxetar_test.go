package image_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/runner"
)

func TestBootcPXETarNoCustomizations(t *testing.T) {
	containerSource := container.SourceSpec{
		Source: "some-src",
		Name:   "name",
	}
	containers := []container.SourceSpec{containerSource}
	img := image.NewBootcPXEImage(testPlatform, "bootc-test-pxe.tar.xz", containerSource, containerSource)
	require.NotNil(t, img)
	img.Compression = "xz"
	img.PartitionTable = testdisk.MakeFakePartitionTable("/", "/boot", "/boot/efi")
	img.KernelVersion = "5.14.0-611.4.1.el9_7.x86_64"

	mf := manifest.New()
	err := img.InstantiateManifestFromContainers(&mf, containers, &runner.Fedora{}, nil)
	require.NoError(t, err)

	fakeSourceSpecs := map[string][]container.Spec{
		"build": []container.Spec{{Source: "some-src", Digest: makeFakeDigest(t), ImageID: makeFakeDigest(t)}},
		"image": []container.Spec{{Source: "other-src", Digest: makeFakeDigest(t), ImageID: makeFakeDigest(t)}},
	}
	osbm, err := mf.Serialize(nil, fakeSourceSpecs, nil, nil, nil)
	require.NoError(t, err)

	// find the tar stage in the tar pipeline
	pipeline := findPipelineFromOsbuildManifest(t, osbm, "tar")
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.tar")
	assert.NotNil(t, stage, "org.osbuild.tar stage not found")

	// check inputs for name:bootc-pxe-tree
	inputs := stage["inputs"].(map[string]any)
	require.NotNil(t, inputs)
	tree := inputs["tree"].(map[string]any)
	require.NotNil(t, tree)
	refstring := tree["references"].([]any)
	require.NotNil(t, refstring)
	require.Equal(t, 1, len(refstring))
	assert.Equal(t, "name:bootc-pxe-tree", refstring[0])

	// check filename for image.tar
	stageOptions := stage["options"].(map[string]any)
	require.NotNil(t, stageOptions)
	assert.Equal(t, "image.tar", stageOptions["filename"].(string))

	// find the xz stage in the xz pipeline
	pipeline = findPipelineFromOsbuildManifest(t, osbm, "xz")
	assert.NotNil(t, pipeline)
	stage = findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.xz")
	assert.NotNil(t, stage, "org.osbuild.xz stage not found")

	// check inputs for name:tar and file: image.tar
	inputs = stage["inputs"].(map[string]any)
	require.NotNil(t, inputs)
	file := inputs["file"].(map[string]any)
	require.NotNil(t, file)
	refmap := file["references"].(map[string]any)
	require.NotNil(t, refmap)
	namepxe := refmap["name:tar"].(map[string]any)
	assert.Equal(t, "image.tar", namepxe["file"].(string))

	// check filename for pxe.tar.xz
	stageOptions = stage["options"].(map[string]any)
	require.NotNil(t, stageOptions)
	assert.Equal(t, "bootc-test-pxe.tar.xz", stageOptions["filename"].(string))
}
