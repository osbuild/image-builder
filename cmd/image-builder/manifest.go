package main

import (
	"io"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
	"github.com/osbuild/image-builder-cli/internal/manifestgen"
)

type manifestOptions struct {
	BlueprintPath string
	Ostree        *ostree.ImageOptions
	RpmDownloader osbuild.RpmDownloader
}

func generateManifest(dataDir string, img *imagefilter.Result, output io.Writer, opts *manifestOptions) error {
	repos, err := newRepoRegistry(dataDir)
	if err != nil {
		return err
	}
	// XXX: add --rpmmd/cachedir option like bib
	mg, err := manifestgen.New(repos, &manifestgen.Options{
		Output:        output,
		RpmDownloader: opts.RpmDownloader,
	})
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
