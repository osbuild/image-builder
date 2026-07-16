package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/distrofactory"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/spf13/cobra"
)

type pkgSearchFormatter interface {
	Output(io.Writer, rpmmd.PackageList) error
}

func newPkgSearchFormatter(format string) (pkgSearchFormatter, error) {
	switch format {
	case "", "json":
		return &jsonPkgFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported format %q (supported: json)", format)
	}
}

type pkgSearchPackageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
	Summary string `json:"summary"`
}

type pkgSearchResultJSON struct {
	Packages []pkgSearchPackageJSON `json:"packages"`
}

type jsonPkgFormatter struct{}

func (*jsonPkgFormatter) Output(w io.Writer, pkgs rpmmd.PackageList) error {
	result := pkgSearchResultJSON{
		Packages: make([]pkgSearchPackageJSON, len(pkgs)),
	}
	for i, p := range pkgs {
		result.Packages[i] = pkgSearchPackageJSON{
			Name:    p.Name,
			Version: p.Version,
			Release: p.Release,
			Arch:    p.Arch,
			Summary: p.Summary,
		}
	}
	enc := json.NewEncoder(w)
	return enc.Encode(result)
}

// pkgSearcher performs the actual package search. It is a variable so
// tests can replace it with a fake that doesn't require osbuild-depsolve-dnf.
var pkgSearcher = func(d distro.Distro, archStr, cacheDir string, repos []rpmmd.RepoConfig, packages []string) (rpmmd.PackageList, error) {
	solver := depsolvednf.NewSolver(d.ModulePlatformID(), d.Releasever(), archStr, d.Name(), cacheDir)
	return solver.SearchMetadata(repos, packages)
}

func cmdPkgSearch(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("at least one package name is required")
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	formatter, err := newPkgSearchFormatter(format)
	if err != nil {
		return err
	}

	repoDir, err := cmd.Flags().GetString("force-repo-dir")
	if err != nil {
		return err
	}

	extraRepos, err := cmd.Flags().GetStringArray("extra-repo")
	if err != nil {
		return err
	}

	forceRepos, err := cmd.Flags().GetStringArray("force-repo")
	if err != nil {
		return err
	}

	distroStr, err := cmd.Flags().GetString("distro")
	if err != nil {
		return err
	}

	distroStr, err = findDistro(distroStr, "")
	if err != nil {
		return err
	}

	archStr, err := cmd.Flags().GetString("arch")
	if err != nil {
		return err
	}

	if archStr == "" {
		archStr = arch.Current().String()
	}

	imageType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}

	repoOpts := repoOptions{
		RepoDir:    repoDir,
		ExtraRepos: extraRepos,
	}

	var d distro.Distro
	var searchRepos []rpmmd.RepoConfig
	if imageType != "" {
		img, err := getOneImage(distroStr, imageType, archStr, &repoOpts)
		if err != nil {
			return err
		}

		d = img.ImgType.Arch().Distro()
		searchRepos = img.Repos
	} else {
		factory := distrofactory.NewDefault()
		d = factory.GetDistro(distroStr)
		if d == nil {
			return fmt.Errorf("unknown distro %q", distroStr)
		}

		registry, err := newRepoRegistry(repoDir, extraRepos)
		if err != nil {
			return err
		}
		searchRepos, err = registry.ReposByArchName(distroStr, archStr, true)
		if err != nil {
			return err
		}
	}

	if len(forceRepos) > 0 {
		searchRepos, err = parseRepoURLs(forceRepos, "force")
		if err != nil {
			return err
		}
	}

	cacheDir, err := cmd.Flags().GetString("rpmmd-cache")
	if err != nil {
		return err
	}

	if cacheDir == "" {
		cacheDir = defaultCacheDir()
	}

	results, err := pkgSearcher(d, archStr, cacheDir, searchRepos, args)
	if err != nil {
		return err
	}

	return formatter.Output(osStdout, results)
}
