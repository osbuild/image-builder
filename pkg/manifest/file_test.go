package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/runner"
)

func testPartitionTable() *disk.PartitionTable {
	return &disk.PartitionTable{
		Type: disk.PT_GPT,
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Partitions: []disk.Partition{
			{
				Start: 1048576,
				Size:  524288000,
				UUID:  "FAC7F1FB-3E8D-4137-A512-961DE09A5549",
				Payload: &disk.Filesystem{
					Type:       "ext4",
					UUID:       "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
					Mountpoint: "/boot",
				},
			},
			{
				Start: 525336576,
				Size:  5368709120,
				UUID:  "CB07C243-BC44-4717-853E-28852021225B",
				Payload: &disk.Filesystem{
					Type:       "xfs",
					UUID:       "a178892e-e285-4ce1-9114-55780875d64e",
					Mountpoint: "/",
				},
			},
		},
	}
}

func TestFileImageSerialize(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	pt := testPartitionTable()
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})

	filePipeline := manifest.NewFileImage(build, rawImage, "/etc/os-release", pt, "file-osrelease")

	pipeline, err := manifest.Serialize(filePipeline)
	require.NoError(t, err)

	assert.Equal(t, "file-osrelease", pipeline.Name)
	require.Equal(t, 2, len(pipeline.Stages))

	// First stage: copy the raw image into the pipeline tree
	copyImageStage := pipeline.Stages[0]
	assert.Equal(t, "org.osbuild.copy", copyImageStage.Type)
	opts := copyImageStage.Options.(*osbuild.CopyStageOptions)
	require.Len(t, opts.Paths, 1)
	assert.Equal(t, "input://image/disk.img", opts.Paths[0].From)
	assert.Equal(t, "tree:///disk.img", opts.Paths[0].To)

	// Second stage: mount the image and copy the file
	copyFileStage := pipeline.Stages[1]
	assert.Equal(t, "org.osbuild.copy", copyFileStage.Type)
	fileOpts := copyFileStage.Options.(*osbuild.CopyStageOptions)
	require.Len(t, fileOpts.Paths, 1)
	assert.Contains(t, fileOpts.Paths[0].From, "/etc/os-release")
	assert.Equal(t, "tree:///os-release", fileOpts.Paths[0].To)
}

func TestFileImageDefaultFilename(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	pt := testPartitionTable()
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	filePipeline := manifest.NewFileImage(build, rawImage, "/boot/vmlinuz", pt, "file-kernel")

	assert.Equal(t, "vmlinuz", filePipeline.Filename())
}

func TestFileImageSetFilename(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	pt := testPartitionTable()
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	filePipeline := manifest.NewFileImage(build, rawImage, "/boot/vmlinuz", pt, "file-kernel")

	filePipeline.SetFilename("kernel.bin")
	assert.Equal(t, "kernel.bin", filePipeline.Filename())
}

func TestFileImageExport(t *testing.T) {
	mani := manifest.New()
	runner := &runner.Linux{}
	build := manifest.NewBuild(&mani, runner, nil, nil)

	pt := testPartitionTable()
	rawImage := manifest.NewRawImage(build, nil, manifest.DiskCustomizations{})
	filePipeline := manifest.NewFileImage(build, rawImage, "/etc/os-release", pt, "file-osrelease")
	filePipeline.SetFilename("os-release.txt")

	art := filePipeline.Export()
	assert.NotNil(t, art)
	assert.Equal(t, "os-release.txt", art.Filename())
}
