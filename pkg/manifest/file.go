package manifest

import (
	"fmt"
	"path/filepath"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// FileImage extracts a single file from an assembled raw disk image by
// mounting its filesystems and copying the file out. It runs after the
// full image (including the bootloader) has been assembled.
type FileImage struct {
	Base
	filename    string
	imgPipeline FilePipeline

	// Path is the absolute path inside the image's filesystem to extract.
	Path string

	// PartitionTable is used to set up loopback devices and mounts.
	PartitionTable *disk.PartitionTable
}

func NewFileImage(buildPipeline Build, imgPipeline FilePipeline, path string, pt *disk.PartitionTable, name string) *FileImage {
	p := &FileImage{
		Base:           NewBase(name, buildPipeline),
		imgPipeline:    imgPipeline,
		filename:       filepath.Base(path),
		Path:           path,
		PartitionTable: pt,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *FileImage) Filename() string {
	return p.filename
}

func (p *FileImage) SetFilename(filename string) {
	p.filename = filename
}

func (p *FileImage) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	// Copy the raw disk image from the input pipeline into this pipeline's
	// tree so it can be loop-mounted.
	imgFilename := "disk.img"
	copyImageOpts := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://image/%s", p.imgPipeline.Filename()),
				To:   fmt.Sprintf("tree:///%s", imgFilename),
			},
		},
	}
	copyImageInputs := osbuild.NewPipelineTreeInputs("image", p.imgPipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStageSimple(copyImageOpts, copyImageInputs))

	// Mount the disk image and copy the file out.
	fsRootMntName, mounts, devices, err := osbuild.GenMountsDevicesFromPT(imgFilename, p.PartitionTable)
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	copyFileOpts := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("mount://%s%s", fsRootMntName, p.Path),
				To:   fmt.Sprintf("tree:///%s", p.filename),
			},
		},
	}
	pipeline.AddStage(osbuild.NewCopyStage(copyFileOpts, nil, devices, mounts))

	return pipeline, nil
}

func (p *FileImage) getBuildPackages(Distro) ([]string, error) {
	return nil, nil
}

func (p *FileImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
