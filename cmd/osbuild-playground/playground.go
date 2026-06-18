package main

import (
	"fmt"
	"math/rand"
	"os"
	"path"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/image"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
	"github.com/osbuild/image-builder/pkg/sbom"
)

func RunPlayground(img image.ImageKind, d distro.Distro, arch distro.Arch, repos map[string][]rpmmd.RepoConfig, state_dir string) {

	solver := depsolvednf.NewSolver(d.ModulePlatformID(), d.Releasever(), arch.Name(), d.Name(), path.Join(state_dir, "rpmmd"))

	// Set cache size to 1 GiB
	solver.SetMaxCacheSize(1 * datasizes.GiB)

	manifest := manifest.New()

	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))

	// TODO: query distro for runner
	artifact, err := img.InstantiateManifest(&manifest, repos[arch.Name()], &runner.Fedora{Version: 36}, rnd)
	if err != nil {
		panic("InstantiateManifest() failed: " + err.Error())
	}

	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)
	for name, chain := range common.Must(manifest.GetPackageSetChains()) {
		res, err := solver.Depsolve(chain, sbom.StandardTypeNone)
		if err != nil {
			panic(fmt.Sprintf("failed to depsolve for pipeline %s: %s\n", name, err.Error()))
		}
		depsolvedSets[name] = *res
	}

	if err := solver.CleanCache(); err != nil {
		// print to stderr but don't exit with error
		fmt.Fprintf(os.Stderr, "could not clean dnf cache: %s", err.Error())
	}

	bytes, err := manifest.Serialize(depsolvedSets, nil, nil, nil, nil)
	if err != nil {
		panic("failed to serialize manifest: " + err.Error())
	}

	store := path.Join(state_dir, "osbuild-store")

	_, err = osbuild.RunOSBuild(bytes, &osbuild.OSBuildOptions{
		StoreDir:    store,
		OutputDir:   "./",
		Exports:     manifest.GetExports(),
		Checkpoints: manifest.GetCheckpoints(),
		JSONOutput:  false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not run osbuild: %s", err.Error())
	}

	fmt.Fprintf(os.Stderr, "built ./%s/%s (%s)\n", artifact.Export(), artifact.Filename(), artifact.MIMEType())
}
