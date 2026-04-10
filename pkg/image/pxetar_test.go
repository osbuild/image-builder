package image_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func TestPXETarNoCustomizations(t *testing.T) {
	img := image.NewPXETar(testPlatform, "test-pxe.tar.xz")
	require.NotNil(t, img)

	img.Compression = "xz"
	img.OSVersion = "42"

	source := rand.NewSource(int64(0))
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	mf := manifest.New()
	_, err := img.InstantiateManifest(&mf, nil, &runner.Fedora{}, rng)
	require.NoError(t, err)

	// Fake package sets to keep serialization happy
	repo := rpmmd.RepoConfig{Id: "dummy-repo-id"}
	pkgSets := map[string]depsolvednf.DepsolveResult{
		"build": {
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name: "coreutils",
						Checksum: rpmmd.Checksum{
							Type:  "sha256",
							Value: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
						},
						RemoteLocations: []string{"https://example.com/coreutils"},
						RepoID:          repo.Id,
						Repo:            &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
		"os": {
			Transactions: depsolvednf.TransactionList{
				{
					{
						Name: "kernel",
						Checksum: rpmmd.Checksum{
							Type:  "sha256",
							Value: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
						},
						RemoteLocations: []string{"https://example.com/kernel"},
						RepoID:          repo.Id,
						Repo:            &repo,
					},
				},
			},
			Repos: []rpmmd.RepoConfig{repo},
		},
	}

	osbm, err := mf.Serialize(pkgSets, nil, nil, nil, nil)
	require.NoError(t, err)

	// find the tar stage in the tar pipeline
	pipeline := findPipelineFromOsbuildManifest(t, osbm, "tar")
	assert.NotNil(t, pipeline)
	stage := findStageFromOsbuildPipeline(t, pipeline, "org.osbuild.tar")
	assert.NotNil(t, stage, "org.osbuild.tar stage not found")

	// check inputs for name:pxe-tree
	inputs := stage["inputs"].(map[string]any)
	require.NotNil(t, inputs)
	tree := inputs["tree"].(map[string]any)
	require.NotNil(t, tree)
	refstring := tree["references"].([]any)
	require.NotNil(t, refstring)
	require.Equal(t, 1, len(refstring))
	assert.Equal(t, "name:pxe-tree", refstring[0])

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
	assert.Equal(t, "test-pxe.tar.xz", stageOptions["filename"].(string))
}
