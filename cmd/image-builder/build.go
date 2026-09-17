package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder/pkg/imagefilter"
	"github.com/osbuild/image-builder/pkg/progress"
)

type buildOptions struct {
	OutputDir      string
	StoreDir       string
	OutputBasename string
	InVm           []string
	JSONOutput     bool

	WriteManifest bool
	WriteBuildlog bool
	Metrics       bool
}

func buildImage(pbar progress.ProgressBar, res *imagefilter.Result, osbuildManifest []byte, opts *buildOptions) (string, error) {
	if opts == nil {
		opts = &buildOptions{}
	}

	basename, err := basenameFor(res, opts.OutputBasename)
	if err != nil {
		return "", err
	}
	if opts.WriteManifest {
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.osbuild-manifest.json", basename))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return "", err
		}
		// #nosec: G306
		if err := os.WriteFile(p, osbuildManifest, 0644); err != nil {
			return "", err
		}
	}

	osbuildOpts := &progress.OSBuildOptions{
		StoreDir:   opts.StoreDir,
		OutputDir:  opts.OutputDir,
		Metrics:    opts.Metrics,
		InVm:       opts.InVm,
		JSONOutput: opts.JSONOutput,
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
	exports := res.ImgType.Exports()

	if err := progress.RunOSBuild(pbar, osbuildManifest, exports, osbuildOpts); err != nil {
		return "", err
	}
	// Rename *sigh*, see https://github.com/osbuild/image-builder/pull/1039
	// for my preferred way. Every frontend to images has to duplicate
	// similar code like this.
	outputData := outputTmplDataFor(res)
	outputTmpl := defaultOutputTmpl
	if opts.OutputBasename != "" {
		outputTmpl = opts.OutputBasename
	}

	// Main artifact, we store this in a map (even if it contains only a single value
	// because we want to follow the same idiom for everything
	mainRefs := map[string]string{exports[0]: basename}

	// Additional exports defined by the image type, we keep track of where our
	// exports come from, basically this is any export not part of another Refs map
	// which right now is everything after the first export but we might have other
	// additional exports in the future that would reduce that overlap.
	// The template here is expanded with additional data related to the multi
	// export so it can use conditionals to use a different name.
	multiRefs := make(map[string]string, len(exports[1:]))
	for _, export := range exports[1:] {
		data := outputData
		data.Multi.Name = export
		name, err := expandOutputTmpl(outputTmpl, data)
		if err != nil {
			return "", err
		}
		multiRefs[export] = name
	}

	var dstName string
	for _, export := range exports {
		pipelineDir := filepath.Join(opts.OutputDir, export)
		entries, err := os.ReadDir(pipelineDir)
		if err != nil {
			return "", fmt.Errorf("cannot read export directory %q: %w", export, err)
		}
		for _, entry := range entries {
			src := filepath.Join(pipelineDir, entry.Name())
			ext := strings.SplitN(entry.Name(), ".", 2)
			artExt := ""
			if len(ext) > 1 {
				artExt = "." + ext[1]
			}
			artName, ok := mainRefs[export]
			if !ok {
				artName = multiRefs[export]
			}
			dst := filepath.Join(opts.OutputDir, artName+artExt)
			if err := os.Rename(src, dst); err != nil {
				return "", fmt.Errorf("cannot rename artifact %q: %w", entry.Name(), err)
			}
			if ok {
				dstName = dst
			}
		}
		_ = os.Remove(pipelineDir)
	}

	return dstName, nil
}
