package main

import (
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
)

type buildOptions struct {
	OutputDir string
	StoreDir  string
}

func buildImage(res *imagefilter.Result, osbuildManifest []byte, opts *buildOptions) error {
	if opts == nil {
		opts = &buildOptions{}
	}

	// XXX: support output filename via commandline (c.f.
	//   https://github.com/osbuild/images/pull/1039)
	if opts.OutputDir == "" {
		opts.OutputDir = outputDirFor(res)
	}

	// XXX: support stremaing via images/pkg/osbuild/monitor.go
	_, err := osbuild.RunOSBuild(osbuildManifest, opts.StoreDir, opts.OutputDir, res.ImgType.Exports(), nil, nil, false, osStderr)
	return err

}
