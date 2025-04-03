package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/manifestgen"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/sbom"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
)

type manifestOptions struct {
	OutputDir      string
	OutputFilename string
	BlueprintPath  string
	Ostree         *ostree.ImageOptions
	RpmDownloader  osbuild.RpmDownloader
	WithSBOM       bool
	CustomSeed     *int64

	ForceRepos            []string
	UseBootstrapContainer bool
}

func sbomWriter(outputDir, filename string, content io.Reader) error {
	p := filepath.Join(outputDir, filename)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, content); err != nil {
		return err
	}
	return nil
}

func generateManifest(dataDir string, extraRepos []string, img *imagefilter.Result, output io.Writer, depsolveWarningsOutput io.Writer, opts *manifestOptions) error {
	repos, err := newRepoRegistry(dataDir, extraRepos)
	if err != nil {
		return err
	}
	// XXX: add --rpmmd/cachedir option like bib
	manifestGenOpts := &manifestgen.Options{
		Output:                 output,
		DepsolveWarningsOutput: depsolveWarningsOutput,
		RpmDownloader:          opts.RpmDownloader,
		UseBootstrapContainer:  opts.UseBootstrapContainer,
		CustomSeed:             opts.CustomSeed,
	}
	if opts.WithSBOM {
		outputDir := basenameFor(img, opts.OutputDir)
		manifestGenOpts.SBOMWriter = func(filename string, content io.Reader, docType sbom.StandardType) error {
			filename = fmt.Sprintf("%s.%s", basenameFor(img, opts.OutputFilename), strings.SplitN(filename, ".", 2)[1])
			return sbomWriter(outputDir, filename, content)
		}
	}
	if len(opts.ForceRepos) > 0 {
		forcedRepos, err := parseRepoURLs(opts.ForceRepos, "forced")
		if err != nil {
			return err
		}
		manifestGenOpts.OverrideRepos = forcedRepos
	}

	mg, err := manifestgen.New(repos, manifestGenOpts)
	if err != nil {
		return err
	}

	bp, err := blueprintload.Load(opts.BlueprintPath)
	if err != nil {
		return err
	}
	var imgOpts *distro.ImageOptions
	if opts.Ostree != nil {
		imgOpts = &distro.ImageOptions{
			OSTree: opts.Ostree,
		}
	}

	return mg.Generate(bp, img.Distro, img.ImgType, img.Arch, imgOpts)
}
