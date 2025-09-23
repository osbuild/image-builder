package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder-cli/pkg/progress"
	"github.com/osbuild/images/pkg/imagefilter"
)

type buildOptions struct {
	OutputDir      string
	StoreDir       string
	OutputBasename string

	WriteManifest bool
	WriteBuildlog bool
	Metrics       bool
}

func buildImage(pbar progress.ProgressBar, res *imagefilter.Result, osbuildManifest []byte, opts *buildOptions) (string, error) {
	if opts == nil {
		opts = &buildOptions{}
	}

	basename := basenameFor(res, opts.OutputBasename)
	if opts.WriteManifest {
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.osbuild-manifest.json", basename))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, osbuildManifest, 0644); err != nil {
			return "", err
		}
	}

	osbuildOpts := &progress.OSBuildOptions{
		StoreDir:  opts.StoreDir,
		OutputDir: opts.OutputDir,
		Metrics:   opts.Metrics,
	}
	if opts.WriteBuildlog {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return "", fmt.Errorf("cannot create buildlog base directory: %w", err)
		}
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.buildlog", basename))
		f, err := os.Create(p)
		if err != nil {
			return "", fmt.Errorf("cannot create buildlog: %w", err)
		}
		defer f.Close()

		osbuildOpts.BuildLog = f
	}
	if err := progress.RunOSBuild(pbar, osbuildManifest, res.ImgType.Exports(), osbuildOpts); err != nil {
		return "", err
	}
	// Rename *sigh*, see https://github.com/osbuild/images/pull/1039
	// for my preferred way. Every frontend to images has to duplicate
	// similar code like this.
	pipelineDir := filepath.Join(opts.OutputDir, res.ImgType.Exports()[0])
	srcName := filepath.Join(pipelineDir, res.ImgType.Filename())
	imgExt := strings.SplitN(res.ImgType.Filename(), ".", 2)[1]
	dstName := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.%v", basename, imgExt))
	if err := os.Rename(srcName, dstName); err != nil {
		return "", fmt.Errorf("cannot rename artifact to final name: %w", err)
	}
	// best effort, remove the now empty pipeline export dir from osbuild
	_ = os.Remove(pipelineDir)

	return dstName, nil
}
