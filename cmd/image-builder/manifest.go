package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
	"github.com/osbuild/image-builder-cli/internal/manifestgen"
)

type manifestOptions struct {
	BlueprintPath  string
	Ostree         *ostree.ImageOptions
	RpmDownloader  osbuild.RpmDownloader
	ExtraArtifacts []string
}

func sbomWriter(outputDir, filename string, content io.Reader) error {
	p := filepath.Join(outputDir, filename)
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

func generateManifest(dataDir string, img *imagefilter.Result, output io.Writer, opts *manifestOptions) error {
	repos, err := newRepoRegistry(dataDir)
	if err != nil {
		return err
	}
	// XXX: add --rpmmd/cachedir option like bib
	manifestGenOpts := &manifestgen.Options{
		Output:        output,
		RpmDownloader: opts.RpmDownloader,
	}
	if slices.Contains(opts.ExtraArtifacts, "sbom") {
		outputDir := outputDirFor(img)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
		manifestGenOpts.SBOMWriter = func(filename string, content io.Reader) error {
			return sbomWriter(outputDir, filename, content)
		}
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
