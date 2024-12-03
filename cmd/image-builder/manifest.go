package main

import (
	"io"

	"github.com/osbuild/images/pkg/imagefilter"

	"github.com/osbuild/image-builder-cli/internal/blueprintload"
	"github.com/osbuild/image-builder-cli/internal/manifestgen"
)

func generateManifest(dataDir, blueprintPath string, res *imagefilter.Result, output io.Writer) error {
	repos, err := newRepoRegistry(dataDir)
	if err != nil {
		return err
	}
	// XXX: add --rpmmd/cachedir option like bib
	mg, err := manifestgen.New(repos, &manifestgen.Options{
		Output: output,
	})
	if err != nil {
		return err
	}
	bp, err := blueprintload.Load(blueprintPath)
	if err != nil {
		return err
	}

	return mg.Generate(bp, res.Distro, res.ImgType, res.Arch, nil)
}
