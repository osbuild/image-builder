package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/osbuild/image-builder-cli/pkg/progress"
	"github.com/osbuild/images/pkg/imagefilter"
)

type buildOptions struct {
	OutputDir string
	StoreDir  string

	WriteManifest bool
	WriteBuildlog bool
}

func buildImage(pbar progress.ProgressBar, res *imagefilter.Result, osbuildManifest []byte, opts *buildOptions) error {
	if opts == nil {
		opts = &buildOptions{}
	}

	if opts.WriteManifest {
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.osbuild-manifest.json", outputNameFor(res)))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, osbuildManifest, 0644); err != nil {
			return err
		}
	}

	osbuildOpts := &progress.OSBuildOptions{
		StoreDir:  opts.StoreDir,
		OutputDir: opts.OutputDir,
	}
	if opts.WriteBuildlog {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return fmt.Errorf("cannot create buildlog base directory: %w", err)
		}
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.buildlog", outputNameFor(res)))
		f, err := os.Create(p)
		if err != nil {
			return fmt.Errorf("cannot create buildlog: %w", err)
		}
		defer f.Close()

		osbuildOpts.BuildLog = f
	}
	return progress.RunOSBuild(pbar, osbuildManifest, res.ImgType.Exports(), osbuildOpts)
}
