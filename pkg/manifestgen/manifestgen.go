package manifestgen

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/flatpak"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/ostree"
	"github.com/osbuild/image-builder/pkg/reporegistry"
	"github.com/osbuild/image-builder/pkg/rpmlist"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/sbom"
)

const (
	defaultDepsolveCacheDir = "osbuild-depsolve-dnf"
)

var (
	ErrContainerArchMismatch = errors.New("requested container architecture does not match resolved container")
)

// Options contains the optional settings for the manifest generation.
// For unset values defaults will be used.
type Options struct {
	// Cachedir specified the rpmmd cache directory (rpm metadata)
	// If unset a default based on the xdgCacheHome is used
	// (e.g. /root/.cache/osbuild-store)
	Cachedir string

	// RpmDownloader allows to switch between the librepo and
	// libcurl download backends.
	RpmDownloader osbuild.RpmDownloader

	// WarningsOutput will receive any warnings that are part of
	// the manifest generation. If it is unset any warnings will
	// generate an error.
	WarningsOutput io.Writer

	// DepsolveWarningsOutput will receive any warnings that are
	// part of the depsolving step. If it is unset output ends up
	// on the default stdout/stderr.
	DepsolveWarningsOutput io.Writer

	// CustomSeed overrides the default rng seed, this is mostly
	// useful for testing
	CustomSeed *int64

	// OverrideRepos overrides the default repository selection.
	// This is mostly useful for testing
	OverrideRepos []rpmmd.RepoConfig

	// Custom "solver" functions, if unset the defaults will be
	// used. Only needed for specialized use-cases.
	Depsolve          DepsolveFunc
	ContainerResolver ContainerResolverFunc
	CommitResolver    CommitResolverFunc
	FlatpakResolver   FlatpakResolverFunc

	// Use the a bootstrap container to buildroot (useful for e.g.
	// cross-arch or cross-distro builds)
	UseBootstrapContainer bool

	RPMListWriter RPMListWriterFunc

	// SBOMPackages makes Generate return the resolved payload RPMs in
	// Result.Packages, for SBOM generation.
	SBOMPackages bool
}

// Generator can generate an osbuild manifest from a given repository
// and options.
type Generator struct {
	cacheDir string

	depsolve               DepsolveFunc
	containerResolver      ContainerResolverFunc
	commitResolver         CommitResolverFunc
	flatpakResolver        FlatpakResolverFunc
	warningsOutput         io.Writer
	depsolveWarningsOutput io.Writer

	reporegistry *reporegistry.RepoRegistry

	rpmDownloader osbuild.RpmDownloader

	customSeed    *int64
	overrideRepos []rpmmd.RepoConfig

	useBootstrapContainer bool
	rpmlistWriter         RPMListWriterFunc
	sbomPackages          bool
}

// New will create a new manifest generator
func New(reporegistry *reporegistry.RepoRegistry, opts *Options) (*Generator, error) {
	if opts == nil {
		opts = &Options{}
	}
	mg := &Generator{
		reporegistry: reporegistry,

		cacheDir:               opts.Cachedir,
		depsolve:               opts.Depsolve,
		containerResolver:      opts.ContainerResolver,
		commitResolver:         opts.CommitResolver,
		rpmDownloader:          opts.RpmDownloader,
		warningsOutput:         opts.WarningsOutput,
		depsolveWarningsOutput: opts.DepsolveWarningsOutput,
		customSeed:             opts.CustomSeed,
		overrideRepos:          opts.OverrideRepos,
		useBootstrapContainer:  opts.UseBootstrapContainer,
		rpmlistWriter:          opts.RPMListWriter,
		sbomPackages:           opts.SBOMPackages,
	}
	if mg.depsolve == nil {
		mg.depsolve = DefaultDepsolve
	}
	if mg.containerResolver == nil {
		mg.containerResolver = func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {
			return container.NewBlockingResolver(archName).ResolveAll(containerSources)
		}
	}
	if mg.commitResolver == nil {
		mg.commitResolver = ostree.ResolveAll
	}
	if mg.flatpakResolver == nil {
		mg.flatpakResolver = flatpak.ResolveAll
	}
	if mg.cacheDir == "" {
		xdgCacheHomeDir, err := xdgCacheHome()
		if err != nil {
			return nil, err
		}
		mg.cacheDir = filepath.Join(xdgCacheHomeDir, defaultDepsolveCacheDir)
	}

	return mg, nil
}

type Result struct {
	Manifest []byte
	Packages []sbom.Package
}

// Generate will generate a new manifest for the given distro/imageType/arch
// combination.
func (mg *Generator) Generate(bp *blueprint.Blueprint, imgType distro.ImageType, imgOpts *distro.ImageOptions) (Result, error) {
	if imgOpts == nil {
		imgOpts = &distro.ImageOptions{}
	}
	imgOpts.UseBootstrapContainer = mg.useBootstrapContainer
	a := imgType.Arch()
	dist := a.Distro()

	var repos []rpmmd.RepoConfig
	if mg.overrideRepos != nil {
		repos = mg.overrideRepos
	} else {
		var err error
		repos, err = mg.reporegistry.ReposByImageTypeName(dist.Name(), a.Name(), imgType.Name())
		if err != nil {
			return Result{}, err
		}
	}
	// To support "user" a.k.a. "3rd party" repositories, these
	// will have to be added to the repos with
	// <repo_item>.PackageSets set to the "payload" pipeline names
	// for the given image type, see e.g. distro/rhel/imagetype.go:Manifest()
	preManifest, warnings, err := imgType.Manifest(bp, *imgOpts, repos, mg.customSeed)
	if err != nil {
		return Result{}, err
	}
	if len(warnings) > 0 {
		warn := strings.Join(warnings, "\n")
		if mg.warningsOutput != nil {
			fmt.Fprint(mg.warningsOutput, warn)
		} else {
			return Result{}, fmt.Errorf("Warnings during manifest creation:\n%v", warn)
		}
	}
	pkgSetChains, err := preManifest.GetPackageSetChains()
	if err != nil {
		return Result{}, err
	}
	solver := depsolvednf.NewSolver(dist.ModulePlatformID(), dist.Releasever(), a.Name(), dist.Name(), mg.cacheDir)
	if dd, ok := dist.(distro.CustomDepsolverDistro); ok {
		// XXX: it would be nice to have access to arch.Arch
		// from distro.Arch but we dont so we have to do without.
		archi := common.Must(arch.FromString(a.Name()))
		customSolver, cleanupFunc, err := dd.Depsolver(mg.cacheDir, archi)
		if err != nil {
			return Result{}, err
		}
		if customSolver != nil {
			solver = customSolver
		}
		defer func() {
			if err := cleanupFunc(); err != nil {
				fmt.Fprintf(mg.warningsOutput, "WARNING: cleanup failed: %v\n", err)
			}
		}()
	}
	depsolved, err := mg.depsolve(solver, mg.cacheDir, mg.depsolveWarningsOutput, pkgSetChains, dist, a.Name())
	if err != nil {
		return Result{}, err
	}
	containerSpecs, err := mg.containerResolver(preManifest.GetContainerSourceSpecs(), a.Name())
	if err != nil {
		return Result{}, err
	}
	for _, specs := range containerSpecs {
		for _, spec := range specs {
			if spec.Arch.String() != a.Name() {
				return Result{}, fmt.Errorf("%w: %q != %q", ErrContainerArchMismatch, spec.Arch, a.Name())
			}
		}
	}

	commitSpecs, err := mg.commitResolver(preManifest.GetOSTreeSourceSpecs())
	if err != nil {
		return Result{}, err
	}

	flatpakSpecs, err := mg.flatpakResolver(preManifest.GetFlatpakSourceSpecs())
	if err != nil {
		return Result{}, err
	}

	opts := &manifest.SerializeOptions{
		RpmDownloader: mg.rpmDownloader,
	}

	mf, err := preManifest.Serialize(depsolved, containerSpecs, commitSpecs, flatpakSpecs, opts)
	if err != nil {
		return Result{}, err
	}

	if mg.rpmlistWriter != nil {
		uniquePackages := make(map[string]rpmmd.Package)
		// XXX: this is very similar to
		// osbuild-composer:jobimpl-osbuild.go, see if code
		// can be shared
		for plName, depsolvedPipeline := range depsolved {
			pipelinePurpose := "unknown"
			switch {
			case slices.Contains(preManifest.PayloadPipelines(), plName):
				pipelinePurpose = "image"
			case slices.Contains(preManifest.BuildPipelines(), plName):
				pipelinePurpose = "buildroot"
			}
			if pipelinePurpose == "image" {
				addUniquePackagesFromPipeline(uniquePackages, depsolvedPipeline)
			}
		}
		if err := writeRPMList(mg.rpmlistWriter, uniquePackages); err != nil {
			return Result{}, err
		}
	}

	var packages []sbom.Package
	if mg.sbomPackages {
		packages = sbomInventory(preManifest.PayloadPipelines(), depsolved)
	}

	return Result{Manifest: mf, Packages: packages}, nil
}

func sbomInventory(payload []string, depsolved map[string]depsolvednf.DepsolveResult) []sbom.Package {
	var packages []sbom.Package
	for _, plName := range payload {
		pipeline, ok := depsolved[plName]
		if !ok {
			continue
		}
		for _, pkg := range pipeline.Transactions.AllPackages() {
			packages = append(packages, sbom.PackageFromRPMMD(pkg))
		}
	}
	return packages
}

func addUniquePackagesFromPipeline(unique map[string]rpmmd.Package, pipeline depsolvednf.DepsolveResult) {
	for _, pkg := range pipeline.Transactions.AllPackages() {
		var key string
		key = pkg.Checksum.Value
		if pkg.Checksum.Value == "" {
			key = fmt.Sprintf("%s-%s-%s.%s", pkg.Name, pkg.Version, pkg.Release, pkg.Arch)
		}
		if _, exists := unique[key]; !exists {
			unique[key] = pkg
		}
	}
}

func writeRPMList(writer RPMListWriterFunc, unique map[string]rpmmd.Package) error {
	var packages rpmmd.PackageList
	for _, pkg := range unique {
		packages = append(packages, pkg)
	}
	rpmListJSON, err := rpmlist.EncodePackages(packages)
	if err != nil {
		return err
	}
	return writer("rpmlist.json", rpmListJSON)
}

func xdgCacheHome() (string, error) {
	xdgCacheHome := os.Getenv("XDG_CACHE_HOME")
	if xdgCacheHome != "" {
		return xdgCacheHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// DefaultDepsolve provides a default implementation for depsolving.
// It should rarely be necessary to use it directly and will be used
// by default by manifestgen (unless overriden)
//
// The customSolver argument can be nil
func DefaultDepsolve(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
	if solver == nil {
		return nil, fmt.Errorf("need a valid solver, got nil")
	}
	if depsolveWarningsOutput != nil {
		solver.Stderr = depsolveWarningsOutput
	}

	return solver.DepsolveAll(packageSets)
}

type (
	DepsolveFunc func(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error)

	ContainerResolverFunc func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error)

	CommitResolverFunc func(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error)

	FlatpakResolverFunc func(flatpakSources map[string][]flatpak.SourceSpec) (map[string][]flatpak.Spec, error)

	RPMListWriterFunc func(filename string, content io.Reader) error
)
