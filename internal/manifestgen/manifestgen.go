package manifestgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// XXX: all of the helpers below are duplicated from
// cmd/build/main.go:depsolve (and probably more places) should go
// into a common helper in "images" or images should do this on its
// own
func defaultDepsolver(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
	if cacheDir == "" {
		var err error
		cacheDir, err = os.MkdirTemp("", "manifestgen")
		if err != nil {
			return nil, fmt.Errorf("cannot create temporary directory: %w", err)
		}
		defer os.RemoveAll(cacheDir)
	}

	solver := depsolvednf.NewSolver(d.ModulePlatformID(), d.Releasever(), arch, d.Name(), cacheDir)
	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)
	for name, pkgSet := range packageSets {
		// XXX: is there harm in always generating an sbom?
		// (expect for slightly longer runtime?)
		res, err := solver.Depsolve(pkgSet, sbom.StandardTypeSpdx)
		if err != nil {
			return nil, fmt.Errorf("error depsolving: %w", err)
		}
		depsolvedSets[name] = *res
	}
	return depsolvedSets, nil
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
	DepsolveFunc func(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error)

	ContainerResolverFunc func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error)

	CommitResolverFunc func(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error)

	SBOMWriterFunc func(filename string, content io.Reader) error
)

// Options contains the optional settings for the manifest generation.
// For unset values defaults will be used.
type Options struct {
	Cachedir string
	Output   io.Writer

	// There are two types of sbom outputs, one for the "payload"
	// and one for the "buildroot", we allow exporting both here
	SbomImageOutput     io.Writer
	SbomBuildrootOutput io.Writer

	Depsolver         DepsolveFunc
	ContainerResolver ContainerResolverFunc
	CommitResolver    CommitResolverFunc

	RpmDownloader osbuild.RpmDownloader

	// Will be called for each generated SBOM the filename
	// contains the suggest filename string and the content
	// can be read
	SBOMWriter SBOMWriterFunc
}

// Generator can generate an osbuild manifest from a given repository
// and options.
type Generator struct {
	cacheDir string
	out      io.Writer

	depsolver         DepsolveFunc
	containerResolver ContainerResolverFunc
	commitResolver    CommitResolverFunc
	sbomWriter        SBOMWriterFunc

	reporegistry *reporegistry.RepoRegistry

	rpmDownloader osbuild.RpmDownloader
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
		rpmDownloader:     opts.RpmDownloader,
		sbomWriter:        opts.SBOMWriter,
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

	repos, err := mg.reporegistry.ReposByImageTypeName(dist.Name(), a.Name(), imgType.Name())
	if err != nil {
		return err
	}
	preManifest, warnings, err := imgType.Manifest(bp, *imgOpts, repos, nil)
	if err != nil {
		return err
	}
	if len(warnings) > 0 {
		// XXX: what can we do here? for things like json output?
		// what are these warnings?
		return fmt.Errorf("warnings during manifest creation: %v", strings.Join(warnings, "\n"))
	}
	depsolved, err := mg.depsolver(mg.cacheDir, preManifest.GetPackageSetChains(), dist, a.Name())
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

	opts := &manifest.SerializeOptions{
		RpmDownloader: mg.rpmDownloader,
	}
	mf, err := preManifest.Serialize(depsolved, containerSpecs, commitSpecs, opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(mg.out, "%s\n", mf)

	if mg.sbomWriter != nil {
		// XXX: this is very similar to
		// osbuild-composer:jobimpl-osbuild.go, see if code
		// can be shared
		for plName, depsolvedPipeline := range depsolved {
			pipelinePurpose := "unknown"
			switch {
			case slices.Contains(imgType.PayloadPipelines(), plName):
				pipelinePurpose = "image"
			case slices.Contains(imgType.BuildPipelines(), plName):
				pipelinePurpose = "buildroot"
			}
			// XXX: sync with image-builder-cli:build.go name generation - can we have a shared helper?
			imageName := fmt.Sprintf("%s-%s-%s", dist.Name(), imgType.Name(), a.Name())
			sbomDocOutputFilename := fmt.Sprintf("%s.%s-%s.spdx.json", imageName, pipelinePurpose, plName)

			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			if err := enc.Encode(depsolvedPipeline.SBOM); err != nil {
				return err
			}
			if err := mg.sbomWriter(sbomDocOutputFilename, &buf); err != nil {
				return err
			}
		}
	}

	return nil
}
