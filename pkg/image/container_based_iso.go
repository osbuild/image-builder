package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
)

type ContainerBasedIso struct {
	Base

	PartitionTable *disk.PartitionTable

	// Customizations
	OSCustomizations        manifest.OSCustomizations
	DiskCustomizations      manifest.DiskCustomizations
	ISOCustomizations       manifest.ISOCustomizations
	InstallerCustomizations manifest.InstallerCustomizations

	// Kernel version from the container, used to copy it into the PXE tar tree
	KernelVersion string

	// Container source for the OS tree
	ContainerSource container.SourceSpec

	KernelOpts []string

	// Custom ISO bootloader menus override the default menus
	Grub2MenuEntries []manifest.ISOGrub2MenuEntry
}

func NewContainerBasedIso(platform platform.Platform, filename string, container container.SourceSpec, buildOpts *manifest.BuildOptions) *ContainerBasedIso {
	return &ContainerBasedIso{
		Base:            NewBase("container-based-iso", platform, filename),
		ContainerSource: container,
		DiskCustomizations: manifest.DiskCustomizations{
			MountConfiguration: osbuild.MOUNT_CONFIGURATION_NONE, // default to no mount config for PXE images
		},
	}
}

// Bootloaders returns the list of configured bootloaders for the platform
func (img *ContainerBasedIso) Bootloaders(buildPipeline manifest.Build, kernelOpts []string) []manifest.ISOBootloader {
	ibo := ISOBootloaders{
		InstallerCustomizations: &img.InstallerCustomizations,
		ISOCustomizations:       &img.ISOCustomizations,
		Custom:                  img.Grub2MenuEntries,
	}

	return ibo.Bootloaders(buildPipeline, img.platform, kernelOpts)
}

func (img *ContainerBasedIso) InstantiateManifestFromContainer(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	cnts := []container.SourceSpec{img.ContainerSource}

	buildOptions := img.BuildOptions
	if buildOptions == nil {
		buildOptions = &manifest.BuildOptions{}
	}
	buildOptions.ContainerBuildable = true
	buildPipeline := manifest.NewBuildFromContainer(m, runner, cnts, buildOptions)

	rawImage := manifest.NewRawBootcImage(buildPipeline, containers, img.platform)
	rawImage.PartitionTable = img.PartitionTable
	rawImage.OSCustomizations = img.OSCustomizations
	rawImage.DiskCustomizations = img.DiskCustomizations
	rawImage.KernelVersion = img.KernelVersion

	// Setup root filesystem so that dmsquash-live will boot it
	rawImage.LiveBoot = true

	rootfsPipeline := manifest.NewBootcRootFS(buildPipeline, rawImage, img.platform)
	rootfsPipeline.ErofsOptions = img.ISOCustomizations.ErofsOptions
	rootfsPipeline.RootfsType = img.ISOCustomizations.RootfsType
	rootfsPipeline.RootfsExcludes = img.ISOCustomizations.RootfsExcludes

	// Setup the boot args for a live iso with ostree rootfs
	// The @OSTREE@ entry is substituted by the org.osbuild.ostree.grub2 stage
	// after inspecting the rootfs.img for the ostree default boot target's path.
	// this requires the ISOTree bool SetOSTREE to be true.
	kernelOpts := []string{
		// TODO escape the label (or make sure it is escaped earlier)
		fmt.Sprintf("root=live:CDLABEL=%s", img.ISOCustomizations.Label),
		"ostree=@OSTREE@",
		"rd.live.image",
		"fstab=no", // Prevents systemd-fstab-generator from making mount units
		"quiet",
		"rhgb",
	}
	kernelOpts = append(kernelOpts, img.KernelOpts...)
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)

	// Setup the bootloaders
	bootloaders := img.Bootloaders(buildPipeline, kernelOpts)

	isoTreePipeline := manifest.NewISOTree(buildPipeline, rootfsPipeline, bootloaders)
	isoTreePipeline.PartitionTable = disk.EFIBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	// Paths are relative to the rootfsPipeline tree root
	isoTreePipeline.KernelPath = "vmlinuz"
	isoTreePipeline.InitramfsPath = "initrd.img"
	isoTreePipeline.RootfsPath = "rootfs.img"
	// Mount the rootfs, get the ostree boot path, update grub.cfg ostree= entry
	isoTreePipeline.SetOSTREE = true
	isoTreePipeline.RootfsType = rootfsPipeline.RootfsType

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)
	artifact := isoPipeline.Export()

	return artifact, nil
}
