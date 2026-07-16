package main

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/image-builder/internal/cmdutil"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/bib/osinfo"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/customizations/anaconda"
	"github.com/osbuild/image-builder/pkg/customizations/kickstart"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/distro/defs"
	"github.com/osbuild/image-builder/pkg/distro/generic"
	"github.com/osbuild/image-builder/pkg/image"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"

	podman_container "github.com/osbuild/image-builder/pkg/bootc"
)

// all possible locations for the bib's distro definitions
// ./data/defs and ./bib/data/defs are for development
// /usr/share/bootc-image-builder/defs is for the production, containerized version
var distroDefPaths = []string{
	"./data/defs",
	"./bib/data/defs",
	"/usr/share/bootc-image-builder/defs",
}

type ManifestConfig struct {
	// OCI image path (without the transport, that is always docker://)
	Imgref      string
	BuildImgref string

	// Build config
	Config *blueprint.Blueprint

	// CPU architecture of the image
	Architecture arch.Arch

	// Paths to the directory with the distro definitions
	DistroDefPaths []string

	// Extracted information about the source container image
	SourceInfo      *osinfo.Info
	BuildSourceInfo *osinfo.Info

	// RootFSType specifies the filesystem type for the root partition
	RootFSType string

	// use librepo ad the rpm downlaod backend
	UseLibrepo bool
}

func manifestFromCobraForLegacyISO(imgref, buildImgref, imgTypeStr, rootFs, rpmCacheRoot string, config *blueprint.Blueprint, useLibrepo bool, cntArch arch.Arch) ([]byte, *mTLSConfig, error) {
	container, err := podman_container.NewContainer(imgref)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := container.Stop(); err != nil {
			logrus.Warnf("error stopping container: %v", err)
		}
	}()

	var rootfsType string
	if rootFs != "" {
		rootfsType = rootFs
	} else {
		bic, err := container.InstallConfiguration()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot get rootfs type for container: %w", err)
		}
		rootfsType = bic.Filesystem.Root.Type
		if rootfsType == "" {
			return nil, nil, fmt.Errorf(`no default root filesystem type specified in container, please use "--rootfs" to set manually`)
		}
	}

	// Gather some data from the containers distro
	sourceinfo, err := osinfo.Load(container.Root())
	if err != nil {
		return nil, nil, err
	}

	buildContainer := container
	buildSourceinfo := sourceinfo
	startedBuildContainer := false
	defer func() {
		if startedBuildContainer {
			if err := buildContainer.Stop(); err != nil {
				logrus.Warnf("error stopping container: %v", err)
			}
		}
	}()

	if buildImgref != "" {
		buildContainer, err = podman_container.NewContainer(buildImgref)
		if err != nil {
			return nil, nil, err
		}
		startedBuildContainer = true

		// Gather some data from the containers distro
		buildSourceinfo, err = osinfo.Load(buildContainer.Root())
		if err != nil {
			return nil, nil, err
		}
	} else {
		buildImgref = imgref
	}

	// This is needed just for RHEL and RHSM in most cases, but let's run it every time in case
	// the image has some non-standard dnf plugins.
	if err := buildContainer.InitDNF(); err != nil {
		return nil, nil, err
	}
	solver, err := buildContainer.NewContainerSolver(rpmCacheRoot, cntArch, sourceinfo)
	if err != nil {
		return nil, nil, err
	}

	manifestConfig := &ManifestConfig{
		Architecture:    cntArch,
		Config:          config,
		Imgref:          imgref,
		BuildImgref:     buildImgref,
		DistroDefPaths:  distroDefPaths,
		SourceInfo:      sourceinfo,
		BuildSourceInfo: buildSourceinfo,
		RootFSType:      rootfsType,
		UseLibrepo:      useLibrepo,
	}

	manifest, repos, err := makeISOManifest(manifestConfig, solver, rpmCacheRoot)
	if err != nil {
		return nil, nil, err
	}

	mTLS, err := extractTLSKeys(repos)
	if err != nil {
		return nil, nil, err
	}

	return manifest, mTLS, nil
}

func makeISOManifest(c *ManifestConfig, solver *depsolvednf.Solver, cacheRoot string) (manifest.OSBuildManifest, map[string][]rpmmd.RepoConfig, error) {
	seed, err := cmdutil.NewRNGSeed()
	if err != nil {
		return nil, nil, err
	}
	//nolint:gosec
	rng := rand.New(rand.NewSource(seed))

	mani, err := manifestForISO(c, rng)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get manifest: %w", err)
	}

	// depsolve packages
	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)
	depsolvedRepos := make(map[string][]rpmmd.RepoConfig)
	pkgSetChains, err := mani.GetPackageSetChains()
	if err != nil {
		return nil, nil, err
	}
	for name, pkgSet := range pkgSetChains {
		res, err := solver.Depsolve(pkgSet, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot depsolve: %w", err)
		}
		depsolvedSets[name] = *res
		depsolvedRepos[name] = res.Repos
	}

	// Resolve container - the normal case is that host and target
	// architecture are the same. However it is possible to build
	// cross-arch images by using qemu-user. This will run everything
	// (including the build-root) with the target arch then, it
	// is fast enough (given that it's mostly I/O and all I/O is
	// run naively via syscall translation)

	// XXX: should NewResolver() take "arch.Arch"?
	resolver := container.NewResolver(c.Architecture.String())

	containerSpecs := make(map[string][]container.Spec)
	for plName, sourceSpecs := range mani.GetContainerSourceSpecs() {
		for _, c := range sourceSpecs {
			resolver.Add(c)
		}
		specs, err := resolver.Finish()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot resolve containers: %w", err)
		}
		for _, spec := range specs {
			if spec.Arch != c.Architecture {
				return nil, nil, fmt.Errorf("image found is for unexpected architecture %q (expected %q), if that is intentional, please make sure --target-arch matches", spec.Arch, c.Architecture)
			}
		}
		containerSpecs[plName] = specs
	}

	var opts manifest.SerializeOptions
	if c.UseLibrepo {
		opts.RpmDownloader = osbuild.RpmDownloaderLibrepo
	}
	mf, err := mani.Serialize(depsolvedSets, containerSpecs, nil, nil, &opts)
	if err != nil {
		return nil, nil, fmt.Errorf("[ERROR] manifest serialization failed: %s", err.Error())
	}
	return mf, depsolvedRepos, nil
}

func labelForISO(os *osinfo.OSRelease, arch *arch.Arch) string {
	switch os.ID {
	case "fedora":
		return fmt.Sprintf("Fedora-S-dvd-%s-%s", arch, os.VersionID)
	case "centos":
		labelTemplate := "CentOS-Stream-%s-BaseOS-%s"
		if os.VersionID == "8" {
			labelTemplate = "CentOS-Stream-%s-%s-dvd"
		}
		return fmt.Sprintf(labelTemplate, os.VersionID, arch)
	case "rhel":
		version := strings.ReplaceAll(os.VersionID, ".", "-")
		return fmt.Sprintf("RHEL-%s-BaseOS-%s", version, arch)
	default:
		return fmt.Sprintf("Container-Installer-%s", arch)
	}
}

// from:https://github.com/osbuild/images/blob/v0.207.0/data/distrodefs/rhel-10/imagetypes.yaml#L169
var loraxRhelTemplates = []manifest.InstallerLoraxTemplate{
	manifest.InstallerLoraxTemplate{Path: "80-rhel/runtime-postinstall.tmpl"},
	manifest.InstallerLoraxTemplate{Path: "80-rhel/runtime-cleanup.tmpl", AfterDracut: true},
}

// from:https://github.com/osbuild/images/blob/v0.207.0/data/distrodefs/fedora/imagetypes.yaml#L408
var loraxFedoraTemplates = []manifest.InstallerLoraxTemplate{
	manifest.InstallerLoraxTemplate{Path: "99-generic/runtime-postinstall.tmpl"},
	manifest.InstallerLoraxTemplate{Path: "99-generic/runtime-cleanup.tmpl", AfterDracut: true},
}

func loraxTemplates(si osinfo.OSRelease) []manifest.InstallerLoraxTemplate {
	switch {
	case si.ID == "rhel" || slices.Contains(si.IDLike, "rhel") || si.VersionID == "eln":
		return loraxRhelTemplates
	default:
		return loraxFedoraTemplates
	}
}

func loraxTemplatePackage(si osinfo.OSRelease) string {
	switch {
	case si.ID == "rhel" || slices.Contains(si.IDLike, "rhel") || si.VersionID == "eln":
		return "lorax-templates-rhel"
	default:
		return "lorax-templates-generic"
	}
}

func manifestForISO(c *ManifestConfig, rng *rand.Rand) (*manifest.Manifest, error) {
	if c.Imgref == "" {
		return nil, fmt.Errorf("pipeline: no base image defined")
	}

	// This gets the installer package set from the distro reported by the bootc container
	distroYAML, id, err := generic.NewDistroYAMLFrom(defs.BuiltinLoader(), c.SourceInfo)
	if err != nil {
		return nil, err
	}

	// Each distro's imagetype.yaml has a bootc-rpm-installer section used for the
	// anaconda-iso package set
	installerImgTypeName := "bootc-rpm-installer"
	imgType, ok := distroYAML.ImageTypes()[installerImgTypeName]
	if !ok {
		return nil, fmt.Errorf("cannot find image definition for %v", installerImgTypeName)
	}
	installerPkgSet, ok := imgType.PackageSets(*id, c.Architecture.String())["installer"]
	if !ok {
		return nil, fmt.Errorf("cannot find installer package set for %v", installerImgTypeName)
	}

	containerSource := container.SourceSpec{
		Source: c.Imgref,
		Name:   c.Imgref,
		Local:  true,
	}

	platform := &platform.Data{
		Arch:        c.Architecture,
		ImageFormat: platform.FORMAT_ISO,
		UEFIVendor:  c.SourceInfo.UEFIVendor,
	}
	switch c.Architecture {
	case arch.ARCH_X86_64:
		platform.BIOSPlatform = "i386-pc"
	case arch.ARCH_AARCH64:
		// aarch64 always uses UEFI, so let's enforce the vendor
		if c.SourceInfo.UEFIVendor == "" {
			return nil, fmt.Errorf("UEFI vendor must be set for aarch64 ISO")
		}
	case arch.ARCH_S390X:
		platform.ZiplSupport = true
	case arch.ARCH_PPC64LE:
		platform.BIOSPlatform = "powerpc-ieee1275"
	case arch.ARCH_RISCV64:
		// nothing special needed
	default:
		return nil, fmt.Errorf("unsupported architecture %v", c.Architecture)
	}
	filename := "install.iso"

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstallerLegacy(platform, filename, containerSource)
	img.ContainerRemoveSignatures = true
	img.RootfsCompression = "zstd"

	if c.Architecture == arch.ARCH_X86_64 {
		img.ISOCustomizations.BootType = manifest.Grub2ISOBoot
	}

	img.InstallerCustomizations.Product = c.SourceInfo.OSRelease.Name
	img.InstallerCustomizations.OSVersion = c.SourceInfo.OSRelease.VersionID

	img.ExtraBasePackages = installerPkgSet

	var customizations *blueprint.Customizations
	if c.Config != nil {
		customizations = c.Config.Customizations
	}

	isoCust, err := customizations.GetISO()
	if err != nil {
		return nil, err
	}

	if isoCust != nil && isoCust.VolumeID != "" {
		img.ISOCustomizations.Label = isoCust.VolumeID
	} else {
		img.ISOCustomizations.Label = labelForISO(&c.SourceInfo.OSRelease, &c.Architecture)
	}
	img.InstallerCustomizations.FIPS = customizations.GetFIPS()
	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.Kickstart.KernelOptionsAppend = append(img.Kickstart.KernelOptionsAppend, kopts.Append)
	}
	img.Kickstart.NetworkOnBoot = true

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return nil, err
	}
	if instCust != nil && instCust.Modules != nil {
		img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules, instCust.Modules.Enable...)
		img.InstallerCustomizations.DisabledAnacondaModules = append(img.InstallerCustomizations.DisabledAnacondaModules, instCust.Modules.Disable...)
	}
	img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules,
		anaconda.ModuleUsers,
		anaconda.ModuleServices,
		anaconda.ModuleSecurity,
		// XXX: get from the imagedefs
		anaconda.ModuleNetwork,
		anaconda.ModulePayloads,
		anaconda.ModuleRuntime,
		anaconda.ModuleStorage,
	)

	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: "default",
	}
	img.InstallerCustomizations.LoraxTemplates = loraxTemplates(c.SourceInfo.OSRelease)
	img.InstallerCustomizations.LoraxTemplatePackage = loraxTemplatePackage(c.SourceInfo.OSRelease)

	// see https://github.com/osbuild/bootc-image-builder/issues/733
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs

	installRootfsType, err := disk.NewFSType(c.RootFSType)
	if err != nil {
		return nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := getDistroAndRunner(c.SourceInfo.OSRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifest(&mf, nil, foundRunner, rng)
	return &mf, err
}

func getDistroAndRunner(osRelease osinfo.OSRelease) (manifest.Distro, runner.Runner, error) {
	switch osRelease.ID {
	case "fedora":
		version, err := strconv.ParseUint(osRelease.VersionID, 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse Fedora version (%s): %w", osRelease.VersionID, err)
		}

		return manifest.DISTRO_FEDORA, &runner.Fedora{
			Version: version,
		}, nil
	case "centos":
		version, err := strconv.ParseUint(osRelease.VersionID, 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse CentOS version (%s): %w", osRelease.VersionID, err)
		}
		r := &runner.CentOS{
			Version: version,
		}
		switch version {
		case 9:
			return manifest.DISTRO_EL9, r, nil
		case 10:
			return manifest.DISTRO_EL10, r, nil
		default:
			logrus.Warnf("Unknown CentOS version %d, using default distro for manifest generation", version)
			return manifest.DISTRO_NULL, r, nil
		}

	case "rhel":
		versionParts := strings.Split(osRelease.VersionID, ".")
		if len(versionParts) != 2 {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("invalid RHEL version format: %s", osRelease.VersionID)
		}
		major, err := strconv.ParseUint(versionParts[0], 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse RHEL major version (%s): %w", versionParts[0], err)
		}
		minor, err := strconv.ParseUint(versionParts[1], 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse RHEL minor version (%s): %w", versionParts[1], err)
		}
		r := &runner.RHEL{
			Major: major,
			Minor: minor,
		}
		switch major {
		case 9:
			return manifest.DISTRO_EL9, r, nil
		case 10:
			return manifest.DISTRO_EL10, r, nil
		default:
			logrus.Warnf("Unknown RHEL version %d, using default distro for manifest generation", major)
			return manifest.DISTRO_NULL, r, nil
		}
	}

	logrus.Warnf("Unknown distro %s, using default runner", osRelease.ID)
	return manifest.DISTRO_NULL, &runner.Linux{}, nil
}
