package manifest

import (
	"fmt"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// ISOTree represents a simplified ISO tree that supports squashfs and erofs
// root filesystems.
type ISOTree struct {
	Base

	Release string
	Product string
	Version string

	PartitionTable *disk.PartitionTable
	bootloaders    []ISOBootloader
	files          []*fsnode.File

	treePipeline TreePipeline

	// Kernel, initramfs, and rootfs paths in the tree pipeline
	KernelPath    string
	InitramfsPath string
	RootfsPath    string

	// Optionally set the ostree= argument (substitute the boot path for @OSTREE@ in grub.cfg)
	SetOSTREE bool
	// What is RootfsPath's file type, so it can be mounted for ostree examination
	RootfsType ISORootfsType
}

func NewISOTree(buildPipeline Build, treePipeline TreePipeline, bootloaders []ISOBootloader) *ISOTree {
	// the pipelines should all belong to the same manifest
	for _, b := range bootloaders {
		if b.Manifest() != nil {
			if b.Manifest() != treePipeline.Manifest() {
				panic("pipelines from different manifests")
			}
		}
	}

	p := &ISOTree{
		Base:         NewBase("bootiso-tree", buildPipeline),
		treePipeline: treePipeline,
		bootloaders:  bootloaders,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *ISOTree) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/images",
			},
			{
				Path: "/images/pxeboot",
			},
			{
				Path: "/LiveOS",
			},
		},
	}))

	// Copy kernel and initramfs
	if p.KernelPath == "" || p.InitramfsPath == "" || p.RootfsPath == "" {
		return osbuild.Pipeline{}, fmt.Errorf("kernel, initramfs, and rootfs paths must be set")
	}

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.KernelPath),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.InitramfsPath),
				To:   "tree:///images/pxeboot/initrd.img",
			},
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.RootfsPath),
				To:   "tree:///LiveOS/rootfs.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	// Add bootloaders
	for _, loader := range p.bootloaders {
		stages, files, err := loader.GetISOBootStages(p.treePipeline.Name(), p.PartitionTable)
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("cannot add ISO bootloader: %w", err)
		}
		pipeline.AddStages(stages...)
		p.files = append(p.files, files...)
	}

	// Optional stage to set the ostree= value in the bootloader
	// This is used for bootc/ostree rootfs images
	if p.SetOSTREE {
		lodevice := osbuild.NewLoopbackDevice(
			&osbuild.LoopbackDeviceOptions{
				Filename: "LiveOS/rootfs.img",
			},
		)
		devices := map[string]osbuild.Device{"disk": *lodevice}
		var mounts []osbuild.Mount
		switch p.RootfsType {
		case ErofsRootfs:
			mounts = []osbuild.Mount{*osbuild.NewErofsMount("-", "disk", "/")}
		case SquashfsRootfs:
			mounts = []osbuild.Mount{*osbuild.NewSquashfsMount("-", "disk", "/")}
		default:
			return osbuild.Pipeline{}, fmt.Errorf("Unknown ISOTree rootfs type: %v", p.RootfsType)
		}

		// Update the grub.cfg with the ostree boot uuid
		ostreeStageOptions := &osbuild.OSTreeGrub2StageOptions{
			Filename: "/EFI/BOOT/grub.cfg",
			Source:   "mount://-/",
		}
		pipeline.AddStage(osbuild.NewOSTreeGrub2MountsStage(ostreeStageOptions, nil, devices, mounts))
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.treePipeline.Platform().GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline, nil
}

func (p *ISOTree) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
