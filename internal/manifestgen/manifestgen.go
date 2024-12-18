package manifestgen

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// XXX: all of the helpers below are duplicated from
// cmd/build/main.go:depsolve (and probably more places) should go
// into a common helper in "images" or images should do this on its
// own
func defaultDepsolver(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string][]rpmmd.PackageSpec, map[string][]rpmmd.RepoConfig, error) {
	if cacheDir == "" {
		var err error
		cacheDir, err = os.MkdirTemp("", "manifestgen")
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create temporary directory: %w", err)
		}
		defer os.RemoveAll(cacheDir)
	}

	solver := depsolvednf.NewSolver(d.ModulePlatformID(), d.Releasever(), arch, d.Name(), cacheDir)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	repoSets := make(map[string][]rpmmd.RepoConfig)
	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet, sbom.StandardTypeNone)
		if err != nil {
			return nil, nil, fmt.Errorf("error depsolving: %w", err)
		}
		depsolvedSets[name] = res.Packages
		repoSets[name] = res.Repos
		// the depsolve result also contains SBOM information,
		// it is currently not used here though
	}
	return depsolvedSets, repoSets, nil
}

func resolveContainers(containers []container.SourceSpec, archName string) ([]container.Spec, error) {
	resolver := container.NewResolver(archName)

	for _, c := range containers {
		resolver.Add(c)
	}

	return resolver.Finish()
}

func defaultContainerResolver(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {
	containerSpecs := make(map[string][]container.Spec, len(containerSources))
	for plName, sourceSpecs := range containerSources {
		specs, err := resolveContainers(sourceSpecs, archName)
		if err != nil {
			return nil, fmt.Errorf("error container resolving: %w", err)
		}
		containerSpecs[plName] = specs
	}
	return containerSpecs, nil
}

func defaultCommitResolver(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error) {
	commits := make(map[string][]ostree.CommitSpec, len(commitSources))
	for name, commitSources := range commitSources {
		commitSpecs := make([]ostree.CommitSpec, len(commitSources))
		for idx, commitSource := range commitSources {
			var err error
			commitSpecs[idx], err = ostree.Resolve(commitSource)
			if err != nil {
				return nil, fmt.Errorf("error ostree commit resolving: %w", err)
			}
		}
		commits[name] = commitSpecs
	}
	return commits, nil
}

type (
	DepsolveFunc func(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string][]rpmmd.PackageSpec, map[string][]rpmmd.RepoConfig, error)

	ContainerResolverFunc func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error)

	CommitResolverFunc func(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error)
)

// Options contains the optional settings for the manifest generation.
// For unset values defaults will be used.
type Options struct {
	Cachedir          string
	Output            io.Writer
	Depsolver         DepsolveFunc
	ContainerResolver ContainerResolverFunc
	CommitResolver    CommitResolverFunc
}

// Generator can generate an osbuild manifest from a given repository
// and options.
type Generator struct {
	cacheDir string
	out      io.Writer

	depsolver         DepsolveFunc
	containerResolver ContainerResolverFunc
	commitResolver    CommitResolverFunc

	reporegistry *reporegistry.RepoRegistry
}

// New will create a new manifest generator
func New(reporegistry *reporegistry.RepoRegistry, opts *Options) (*Generator, error) {
	if opts == nil {
		opts = &Options{}
	}
	mg := &Generator{
		reporegistry: reporegistry,

		cacheDir:          opts.Cachedir,
		out:               opts.Output,
		depsolver:         opts.Depsolver,
		containerResolver: opts.ContainerResolver,
		commitResolver:    opts.CommitResolver,
	}
	if mg.out == nil {
		mg.out = os.Stdout
	}
	if mg.depsolver == nil {
		mg.depsolver = defaultDepsolver
	}
	if mg.containerResolver == nil {
		mg.containerResolver = defaultContainerResolver
	}
	if mg.commitResolver == nil {
		mg.commitResolver = defaultCommitResolver
	}

	return mg, nil
}

// Generate will generate a new manifest for the given distro/imageType/arch
// combination.
func (mg *Generator) Generate(bp *blueprint.Blueprint, dist distro.Distro, imgType distro.ImageType, a distro.Arch, imgOpts *distro.ImageOptions) error {
	if imgOpts == nil {
		imgOpts = &distro.ImageOptions{}
	}
	// we may allow to customize the seed in the future via imgOpts or
	// an environment variable
	// XXX: look into "images" so that it automatically seeds when pasing
	// a "0" seed.
	seed := time.Now().UnixNano()

	repos, err := mg.reporegistry.ReposByImageTypeName(dist.Name(), a.Name(), imgType.Name())
	if err != nil {
		return err
	}
	preManifest, warnings, err := imgType.Manifest(bp, *imgOpts, repos, seed)
	if err != nil {
		return err
	}
	if len(warnings) > 0 {
		// XXX: what can we do here? for things like json output?
		// what are these warnings?
		return fmt.Errorf("warnings during manifest creation: %v", strings.Join(warnings, "\n"))
	}
	packageSpecs, _, err := mg.depsolver(mg.cacheDir, preManifest.GetPackageSetChains(), dist, a.Name())
	if err != nil {
		return err
	}
	containerSpecs, err := mg.containerResolver(preManifest.GetContainerSourceSpecs(), a.Name())
	if err != nil {
		return err
	}
	commitSpecs, err := mg.commitResolver(preManifest.GetOSTreeSourceSpecs())
	if err != nil {
		return err
	}
	mf, err := preManifest.Serialize(packageSpecs, containerSpecs, commitSpecs, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(mg.out, "%s\n", mf)

	return nil
}
