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

func generateManifest(dataDir, blueprintPath string, res *imagefilter.Result, output io.Writer, ostreeOpts *ostree.ImageOptions, rpmDownloader osbuild.RpmDownloader) error {
	repos, err := newRepoRegistry(dataDir)
	if err != nil {
		return err
	}
	// XXX: add --rpmmd/cachedir option like bib
	mg, err := manifestgen.New(repos, &manifestgen.Options{
		Output:        output,
		RpmDownloader: rpmDownloader,
	})
	if err != nil {
		return err
	}
	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return err
	}
	var imgOpts *distro.ImageOptions
	if ostreeOpts != nil {
		imgOpts = &distro.ImageOptions{
			OSTree: ostreeOpts,
		}
	}

	return mg.Generate(bp, res.Distro, res.ImgType, res.Arch, imgOpts)
}
