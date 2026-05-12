package image

import (
	"math/rand"

	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
)

// common struct that all anaconda installers share
type AnacondaInstallerBase struct {
	InstallerCustomizations manifest.InstallerCustomizations
	ISOCustomizations       manifest.ISOCustomizations
	RootfsCompression       string

	Kickstart                    *kickstart.Options
	InteractiveDefaultsKickstart *kickstart.Options
}

func initIsoTreePipeline(isoTreePipeline *manifest.AnacondaInstallerISOTree, img *AnacondaInstallerBase, rng *rand.Rand) {
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.Kickstart = img.Kickstart

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.ISOCustomizations.RootfsType

	isoTreePipeline.ISOBoot = img.ISOCustomizations.BootType
}

func (img *AnacondaInstallerBase) Bootloaders(buildPipeline manifest.Build, platform platform.Platform, kernelOpts []string) []manifest.ISOBootloader {
	ibo := ISOBootloaders{
		InstallerCustomizations: &img.InstallerCustomizations,
		ISOCustomizations:       &img.ISOCustomizations,
	}
	return ibo.Bootloaders(buildPipeline, platform, kernelOpts)
}
