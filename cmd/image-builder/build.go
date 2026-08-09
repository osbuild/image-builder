package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/imagefilter"
	"github.com/osbuild/image-builder/pkg/progress"
)

type buildOptions struct {
	OutputDir      string
	StoreDir       string
	OutputBasename string
	InVm           []string
	JSONOutput     bool
	WithExtras     []string

	WriteManifest bool
	WriteBuildlog bool
	Metrics       bool
}

func buildImage(pbar progress.ProgressBar, res *imagefilter.Result, osbuildManifest []byte, opts *buildOptions) ([]string, error) {
	if opts == nil {
		opts = &buildOptions{}
	}

	basename, err := basenameFor(res, opts.OutputBasename)
	if err != nil {
		return nil, err
	}
	if opts.WriteManifest {
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.osbuild-manifest.json", basename))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return nil, err
		}
		// #nosec: G306
		if err := os.WriteFile(p, osbuildManifest, 0644); err != nil {
			return nil, err
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
			return nil, fmt.Errorf("cannot create buildlog base directory: %w", err)
		}
		p := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.buildlog", basename))
		f, err := os.Create(p)
		if err != nil {
			return nil, fmt.Errorf("cannot create buildlog: %w", err)
		}
		defer f.Close()

		osbuildOpts.BuildLog = f
	}
	exports := res.ImgType.Exports()
	var extraRefs map[string]distro.ExtraRef
	if len(opts.WithExtras) > 0 {
		var err error
		exports, extraRefs, err = res.ImgType.ExportsWithExtras(opts.WithExtras)
		if err != nil {
			return nil, err
		}
	}

	if err := progress.RunOSBuild(pbar, osbuildManifest, exports, osbuildOpts); err != nil {
		return nil, err
	}
	// Rename *sigh*, see https://github.com/osbuild/image-builder/pull/1039
	// for my preferred way. Every frontend to images has to duplicate
	// similar code like this.
	outputData := outputTmplDataFor(res)
	outputTmpl := defaultOutputTmpl
	if opts.OutputBasename != "" {
		outputTmpl = opts.OutputBasename
	}

	// Multiple export pipelines from the same extra share a base name;
	// they must produce files with different extensions to avoid collisions.
	names := map[string]string{exports[0]: basename}
	for _, export := range exports[1:] {
		data := outputData
		ref, ok := extraRefs[export]
		if !ok {
			// getImage rejects multiple exports unless --with-extra is set,
			// and ExportsWithExtras populates extraRefs for every pipeline
			// it adds, so this should never be reached.
			return nil, fmt.Errorf("unexpected export pipeline %q without extra reference", export)
		}
		data.Extra.Type = ref.Type
		data.Extra.Name = ref.Name
		name, err := expandOutputTmpl(outputTmpl, data)
		if err != nil {
			return nil, err
		}
		names[export] = name
	}

	var outputPaths []string
	for _, export := range exports {
		pipelineDir := filepath.Join(opts.OutputDir, export)
		entries, err := os.ReadDir(pipelineDir)
		if err != nil {
			return nil, fmt.Errorf("cannot read export directory %q: %w", export, err)
		}
		for _, entry := range entries {
			src := filepath.Join(pipelineDir, entry.Name())
			ext := strings.SplitN(entry.Name(), ".", 2)
			artExt := ""
			if len(ext) > 1 {
				artExt = "." + ext[1]
			}
			dst := filepath.Join(opts.OutputDir, names[export]+artExt)
			if err := os.Rename(src, dst); err != nil {
				return nil, fmt.Errorf("cannot rename artifact %q: %w", entry.Name(), err)
			}
			outputPaths = append(outputPaths, dst)
		}
		_ = os.Remove(pipelineDir)
	}

	return outputPaths, nil
}
