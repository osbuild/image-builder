package main

import (
	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
)

func buildImage(res *imagefilter.Result, osbuildManifest []byte, osbuildStoreDir, outputDir string) error {
	// XXX: support output filename via commandline (c.f.
	//   https://github.com/osbuild/images/pull/1039)
	if outputDir == "" {
		outputDir = outputDirFor(res)
	}

	// XXX: support stremaing via images/pkg/osbuild/monitor.go
	_, err := osbuild.RunOSBuild(osbuildManifest, osbuildStoreDir, outputDir, res.ImgType.Exports(), nil, nil, false, osStderr)
	return err

}
